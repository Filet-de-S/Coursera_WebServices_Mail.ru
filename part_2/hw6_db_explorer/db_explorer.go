package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

type handler struct {
	db		*sql.DB
	tables	*tableStruct
	url		[]string
}

type tableStruct map[string]*table

type table struct {
	primaryKey		string
	primaryKeyType	string
	columns			[]*column
}

type column struct {
	name      string
	fieldType string
	primary   bool
	nullable  bool
	autoInc   bool
	def		  interface{}
}

type reply struct {
	Response interface{} `json:"response,omitempty"`
	Error interface{} `json:"error,omitempty"`
}

type record map[string]interface{}

func NewDbExplorer(db *sql.DB) (http.Handler, error) {
	h := &handler{db: db}
	var err error
	h.tables, err = getTableMap(db); if err != nil {
		panic("cant init table:" + err.Error())
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.url, err = parseUrl(r.URL.Path); err != nil {
			fmt.Println("PARSEURLBEGIN")
			w.WriteHeader(500)
			return
		}

		switch r.Method {
		case "GET":
			h.getHandler(w, r)
		case "PUT":
			//PUT /$table - создаёт новую запись, данный по записи в теле запроса (POST-параметры)
			h.putHandler(w, r)
		case "POST":
			//POST /$table/$id - обновляет запись, данные приходят в теле запроса (POST-параметры)
			h.postHandler(w, r)
		case "DELETE":
			//DELETE /$table/$id - удаляет запись
			h.delHandler(w, r)
		default:
			fmt.Println("HM")
			w.WriteHeader(500)
		}
	}), nil
}

func getTableMap (db *sql.DB) (*tableStruct, error) {
	rows, err := db.Query(`SHOW TABLES`)
	if err != nil {
		rows.Close()
		return nil, err
	}
	tables := tableStruct{}

	for rows.Next() {
		name := ""
		err := rows.Scan(&name)
		if err != nil {
			rows.Close()
			return nil, err
		}
		tables[name] = &table{}
	}
	rows.Close()

	for name := range tables {
		rows, err := db.Query(`SHOW FULL COLUMNS FROM ` + name)
		if err != nil {
			rows.Close()
			return nil, err
		}
		columnNames, err := rows.Columns()
		if err != nil {
			rows.Close()
			return nil, err
		}

		ptrs := make([]interface{}, len(columnNames))
		for i, _ := range columnNames {
			ptrs[i] = new(sql.RawBytes)
		}
		for rows.Next() {
			err = rows.Scan(ptrs...)
			if err != nil {
				rows.Close()
				return nil, err
			}

			clm := &column{}
			for i, rowVal := range ptrs {
				value := string(*rowVal.(*sql.RawBytes))

				switch columnNames[i] {
				case "Field":
					clm.name = value
				case "Type":
					if strings.Contains(value, "varchar") || strings.Contains(value, "text") {
						clm.fieldType = "string"
					} else if strings.Contains(value, "float") {
						clm.fieldType = "float"
					} else {
						clm.fieldType = "int"
					}
				case "Key":
					if value != "" {
						clm.primary = true
					}
				case "Null":
					if value == "YES" {
						clm.nullable = true
					}
				case "Extra":
					if value == "auto_increment" {
						clm.autoInc = true
					}
				case "Default":
					clm.def = value
					fmt.Println("DEF IS:", value)
				}
			}
			if clm.primary {
				tables[name].primaryKey = clm.name
				tables[name].primaryKeyType = clm.fieldType
			}
			tables[name].columns = append(tables[name].columns, clm)

			//rowProperties := make(map[string]string, len(columnNames))
			//for i, rowVal := range ptrs {
			//	rowProperties[columnNames[i]] = string(*rowVal.(*sql.RawBytes))
			//}
			//
			//rowsFromTable := tables[name]
			//rowsFromTable[rowProperties["Field"]] = rowProperties
			//tables[name] = rowsFromTable
		}
		rows.Close()
	}
	return &tables, nil
}

func (fields *table) getItems(keys map[string]interface{}, method string) ([]string, []interface{}, error) {
	names := []string{}
	values := []interface{}{}
	for i := range fields.columns {
		if reqValue, ok := keys[ fields.columns[i].name ]; ok {
			val, ok := fields.columns[i].getTypedValue(reqValue)
			if !ok {
				fmt.Println("getTyped")
				return nil, nil, errors.New(`{"error": "field ` + fields.columns[i].name + ` have invalid type"}`)
			}
			if fields.columns[i].primary {
				if method == "POST" {
					fmt.Println("primary")
					return nil, nil, errors.New(`{"error": "field ` + fields.columns[i].name + ` have invalid type"}`)
				}
				continue
			}
			switch val.(type) {
			case string:
				names = append(names, fields.columns[i].name)
				values = append(values, escape(val.(string)))
			default:
				names = append(names, fields.columns[i].name)
				values = append(values, val)
			}
		} else if !fields.columns[i].nullable && method != "POST" {
			fmt.Println("ohh.", fields.columns[i].name, "cant be null!")
			switch fields.columns[i].def.(type) {
			case string:
				val := fields.columns[i].def.(string)
				names = append(names, fields.columns[i].name)
				switch fields.columns[i].fieldType {
				case "int":
					values = append(values, 0)
				case "float64":
					values = append(values, float64(0))
				default:
					values = append(values, val)
				}
			case float64:
				val := fields.columns[i].def.(float64)
				names = append(names, fields.columns[i].name)
				values = append(values, val)
			case int:
				val := fields.columns[i].def.(int)
				names = append(names, fields.columns[i].name)
				values = append(values, val)
			}
			//return nil, nil, errors.New(`{"error": "field `+fields.columns[i].name+` have invalid type"}`)
		}
	}
	return names, values, nil
}

func (h *handler) putHandler(w http.ResponseWriter, r *http.Request) {

	//PUT /$table - создаёт новую запись, данный по записи в теле запроса (POST-параметры)
	tableQuery := escape(h.url[0])
	fields, ok := (*h.tables)[tableQuery]; if !ok {
		http.Error(w, `{"error": "unknown table"}`, http.StatusNotFound)
		return
	}

	decoder := json.NewDecoder(r.Body)
	var keys map[string]interface{}
	err := decoder.Decode(&keys)
	if err != nil {
		w.WriteHeader(500)
		return
	}

	names, items, err := fields.getItems(keys, "GET"); if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	stmt := getInsertStmt(tableQuery, names)
	res, err := h.db.Exec(stmt, items...)
	if err != nil {
		fmt.Println("something wrong with exec?", err)
		fmt.Println(stmt)
		w.WriteHeader(500)
		return
	}

	lastID, err := res.LastInsertId()
	if err != nil {
		fmt.Println("something wrong with lastID?", err)
		w.WriteHeader(500)
		return
	}

	reply := reply{}
	reply.Response = record{
		fields.primaryKey: lastID,
	}

	result, err := json.Marshal(reply)
	if err != nil {
		errString := `{"error": "cant pack json"}`
		fmt.Println(errString)
		http.Error(w, errString, http.StatusInternalServerError)
		return
	}
	w.Write(result)
}
func getInsertStmt(tableQuery string, names []string) string {
	stmt := `INSERT INTO ` + tableQuery + ` (`
	for i := range names {
		if i < len(names)-1 {
			stmt += "`" + names[i] + "`, "
		} else {
			stmt += "`" + names[i] + "`) "
		}
	}
	stmt += `VALUES(`
	for i := range names {
		if i < len(names)-1 {
			stmt += "?, "
		} else {
			stmt += "?)"
		}
	}
	return stmt
}

func (h *handler) postHandler(w http.ResponseWriter, r *http.Request) {
	//POST /$table/$id - обновляет запись, данные приходят в теле запроса (POST-параметры)
	tableQuery := escape(h.url[0])
	id := escape(h.url[1])
	fields, ok := (*h.tables)[tableQuery]; if !ok {
		fmt.Println("table..")
		http.Error(w, `{"error": "unknown table"}`, http.StatusNotFound)
		return
	} else if !checkPrimary(fields.primaryKeyType, id) {
		fmt.Println("id..")
		http.Error(w, `{"error": "field `+id+` have invalid type"}`, http.StatusBadRequest)
		return
	}

	decoder := json.NewDecoder(r.Body)
	var keys map[string]interface{}
	err := decoder.Decode(&keys)
	if err != nil {
		w.WriteHeader(500)
		return
	}

	names, items, err := fields.getItems(keys, "POST"); if err != nil {
		fmt.Println("items")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	stmt := getUpdateStmt(tableQuery, fields.primaryKey, id, names)
	res, err := h.db.Exec(stmt, items...)
	if err != nil {
		fmt.Println("something wrong with exec?", err)
		w.WriteHeader(500)
		return
	}

	affectedID, err := res.RowsAffected()
	if err != nil {
		fmt.Println("something wrong with lastID?", err)
		w.WriteHeader(500)
		return
	}
	reply := reply{}
	reply.Response = record{
		"updated": affectedID,
	}

	result, err := json.Marshal(reply)
	if err != nil {
		errString := `{"error": "cant pack json"}`
		http.Error(w, errString, http.StatusInternalServerError)
		return
	}
	w.Write(result)
}
func getUpdateStmt(tableQuery, id, idValue string, names []string) string {
	stmt := `UPDATE `+tableQuery+` SET `
	for i := range names {
		if i < len(names)-1 {
			stmt += "`"+names[i]+"` = ?, "
		} else {
			stmt += "`"+names[i]+"` = ? "
		}
	}
	stmt += `WHERE `+id+` = `+idValue
	return stmt
}

func (h *handler) delHandler(w http.ResponseWriter, r *http.Request) {
	//DELETE /$table/$id - удаляет запись
	tableQuery := escape(h.url[0])
	id := escape(h.url[1])
	fields, ok := (*h.tables)[tableQuery]; if !ok {
		http.Error(w, `{"error": "unknown table"}`, http.StatusNotFound)
		return
	} else if !checkPrimary(fields.primaryKeyType, id) {
		http.Error(w, `{"error": "field `+id+` have invalid type"}`, http.StatusBadRequest)
		return
	}

	stmt := `DELETE FROM `+tableQuery+` WHERE `+fields.primaryKey+` = ?`
	res, err := h.db.Exec(stmt, id)
	if err != nil {
		fmt.Println("something wrong with exec?", err)
		w.WriteHeader(500)
		return
	}

	affectedID, err := res.RowsAffected()
	if err != nil {
		fmt.Println("something wrong with affectedID?", err)
		w.WriteHeader(500)
		return
	}
	reply := reply{}
	reply.Response = record{
		"deleted": affectedID,
	}

	result, err := json.Marshal(reply)
	if err != nil {
		errString := `{"error": "cant pack json"}`
		http.Error(w, errString, http.StatusInternalServerError)
		return
	}
	w.Write(result)
}

// GET start
func (h *handler) getHandler(w http.ResponseWriter, r *http.Request) {
	switch {
	case h.url == nil:
		//GET / - возвращает список все таблиц (которые мы можем использовать в дальнейших запросах)
		h.listTables(w, r)
	case len(h.url) == 2:
		//GET /$table/$id - возвращает информацию о самой записи или 404
		h.getEntry(w, r)
	case len(h.url) == 1:
		//GET /$table?limit=5&offset=7 - возвращает список из 5 записей (limit) начиная с 7-й (offset)
		//    из таблицы $table. limit по-умолчанию 5, offset 0
		h.listEntries(w, r)
	default:
		w.WriteHeader(500)
	}
}

func (h *handler) listTables(w http.ResponseWriter, r *http.Request) {
	//GET / - возвращает список все таблиц (которые мы можем использовать в дальнейших запросах)
	reply := reply{}
	tables := []string{}
	for tableName := range *h.tables {
		tables = append(tables, tableName)
	}
	sort.Strings(tables)
	reply.Response = map[string][]string{
		"tables": tables}

	result, err := json.Marshal(reply); if err != nil {
		errString := `{"error": "cant pack json"}`
		http.Error(w, errString, http.StatusInternalServerError)
		return
	}
	w.Write(result)
}

func (h *handler) getEntry(w http.ResponseWriter, r *http.Request) {
	//GET /$tableQuery/$id - возвращает информацию о самой записи или 404
	tableQuery := escape(h.url[0])
	id := h.url[1]
	fields, ok := (*h.tables)[tableQuery]; if !ok {
		fmt.Println("table")
		http.Error(w, `{"error": "unknown table"}`, http.StatusNotFound)
		return
	} else if !checkPrimary(fields.primaryKeyType, id) {
		fmt.Println("invalid type")
		http.Error(w, `{"error": "field `+id+` have invalid type"}`, http.StatusBadRequest)
		return
	}

	row := h.db.QueryRow(`SELECT * FROM `+tableQuery+` WHERE `+fields.primaryKey+` = ?`, id)
	ptrs := makePtrs(fields.columns)

	if err := row.Scan(ptrs...); err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, `{"error": "record not found"}`, http.StatusNotFound)
			return
		}
		fmt.Println("scan err...", err)
		fmt.Println(`SELECT * FROM `+tableQuery+` WHERE `+fields.primaryKey+` = `+id)
		w.WriteHeader(404)
		return
	}

	rcrd := extractRecord(ptrs, fields.columns)
	reply := reply{}
	reply.Response = record{
		"record": *rcrd}

	result, err := json.Marshal(reply); if err != nil {
		errString := `{"error": "cant pack json"}`
		http.Error(w, errString, http.StatusInternalServerError)
		return
	}
	w.Write(result)
}

func (h *handler) listEntries(w http.ResponseWriter, r *http.Request) {
	//GET /$table?limit=5&offset=7 - возвращает список из 5 записей (limit) начиная с 7-й (offset) из таблицы $table. limit по-умолчанию 5, offset 0
	tableQuery := escape(h.url[0])
	fields, ok := (*h.tables)[tableQuery]; if !ok {
		http.Error(w, `{"error": "unknown table"}`, http.StatusNotFound)
		return
	}

	keys := r.URL.Query()
	limit := 5
	offset := 0
	if keys != nil {
		lim, ok := keys["limit"]; if ok {
			if l, err := strconv.Atoi(lim[0]); err == nil {
				limit = l
			}
		}
		offst, ok := keys["offset"]; if ok {
			if o, err := strconv.Atoi(offst[0]); err == nil {
				offset = o
			}
		}
	}

	stmt := `SELECT * FROM `+tableQuery+` LIMIT `+strconv.Itoa(limit)+` OFFSET `+strconv.Itoa(offset)
	fmt.Println(stmt)
	rows, err := h.db.Query(stmt)
	if err != nil {
		rows.Close()
		w.WriteHeader(404)
		return
	}

	rcrds := []record{}
	for rows.Next() {
		ptrs := makePtrs(fields.columns)

		if err := rows.Scan(ptrs...); err != nil {
			if err == sql.ErrNoRows {
				rows.Close()
				http.Error(w, `{"error": "record not found"}`, http.StatusNotFound)
				return
			}
			rows.Close()
			fmt.Println("scan err...", err)
			w.WriteHeader(404)
			return
		}

		rcrd := extractRecord(ptrs, fields.columns)
		rcrds = append(rcrds, *rcrd)
	}
	rows.Close()
	reply := reply{}
	reply.Response = record{
		"records": rcrds,
	}

	result, err := json.Marshal(reply); if err != nil {
		errString := `{"error": "cant pack json"}`
		http.Error(w, errString, http.StatusInternalServerError)
		return
	}
	w.Write(result)
}
// GET end

func makePtrs(columns []*column) []interface{} {
	ptrs := make([]interface{}, len(columns))
	for i := range columns {
		if columns[i].nullable {
			switch columns[i].fieldType {
			case "int":
				ptrs[i] = new(sql.NullInt32)
			case "string":
				ptrs[i] = new(sql.NullString)
			case "float":
				ptrs[i] = new(sql.NullFloat64)
			}
		} else {
			switch columns[i].fieldType {
			case "int":
				ptrs[i] = new(int32)
			case "string":
				ptrs[i] = new(string)
			case "float":
				ptrs[i] = new(float64)
			}
		}
	}
	return ptrs
}

func extractRecord(ptrs []interface{}, columns []*column) *record {
	record := record{}
	for i := range ptrs {
		if columns[i].nullable {
			switch columns[i].fieldType {
			case "int":
				if val := ptrs[i].(*sql.NullInt32); val.Valid {
					record[columns[i].name] = val.Int32
				} else {
					record[columns[i].name] = nil
				}
			case "string":
				if val := ptrs[i].(*sql.NullString); val.Valid {
					record[columns[i].name] = descape(val.String)
				} else {
					record[columns[i].name] = nil
				}
			case "float":
				if val := ptrs[i].(*sql.NullFloat64); val.Valid {
					record[columns[i].name] = val.Float64
				} else {
					record[columns[i].name] = nil
				}
			}
		} else {
			switch columns[i].fieldType {
			case "int":
				record[columns[i].name] = *ptrs[i].(*int32)
			case "string":
				record[columns[i].name] = descape(*ptrs[i].(*string))
			case "float":
				record[columns[i].name] = *ptrs[i].(*float64)
			}
		}
	}
	return &record
}

func checkPrimary (primaryKeyType, id string) bool {
	switch primaryKeyType {
	case "int":
		if _, err := strconv.Atoi(id); err != nil {
			fmt.Println("atoi.......")
			return false
		}
	case "float":
		if _, err := strconv.ParseFloat(id, 64); err != nil {
			fmt.Println("float.......")
			return false
		}
	}
	return true
}

func escape(s string) string {
	str := []byte(s)
	// \0   An ASCII NUL (0x00) character.
	// \'   A single quote (“'”) character.
	// \"   A double quote (“"”) character.
	// \b   A backspace character.
	// \n   A newline (linefeed) character.
	// \r   A carriage return character.
	// \t   A tab character.
	// \Z   ASCII 26 (Control-Z). See note following the table.
	// \\   A backslash (“\”) character.
	// \%   A “%” character. See note following the table.
	// \_   A “_” character. See note following the table.
	for i := 0; i < len(str); i++ {
		switch str[i] {
		case 0x00, '\'', '"', '\b', '\n', '\r', '\t', 26, '\\', '%', '_':
			{
				left := str[:i]
				right := str[i:]
				str = append(left,
					append([]byte(`\`), right...)...)
				i++
			}
		}
	}
	return string(str)
}

func descape(s string) string {
	str := []byte(s)
	for i := 0; i < len(str); i++ {
		if str[i] == '\\' {
			left := str[:i]
			right := []byte{}
			if i+1 < len(str) {
				right = str[i+1:]
			}
			str = append(left, right...)
			i++
		}
	}
	return string(str)
}

func parseUrl(path string) ([]string, error) {
	if path == "/" || path == "" {
		return nil, nil
	}
	paths := strings.Split(path, "/")[1:]
	for i := range paths {
		if paths[i] == "" && i != len(paths)-1 {
			return nil, errors.New("")
		} else if paths[i] == "" {
			return paths[:i], nil
		}
	}
	return paths, nil
}

func (c column) getTypedValue(value interface{}) (interface{}, bool) {
	if value == nil && c.nullable {
		switch c.fieldType {
		case "int":
			return sql.NullInt64{}, true
		case "float":
			return sql.NullFloat64{}, true
		case "string":
			return sql.NullString{}, true
		}
	}

	switch c.fieldType {
	case "int":
		if val, err := value.(float64); err {
			return int(val), true
		}
	case "float":
		if val, err := value.(float64); err {
			return val, true
		}
	case "string":
		if val, err := value.(string); err {
			return val, true
		}
	}
	return nil, false
}

