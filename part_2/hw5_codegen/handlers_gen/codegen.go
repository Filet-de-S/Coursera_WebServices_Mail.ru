// go build handlers_gen/* && ./codegen api.go apiWrapper.go

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

type methodProperties struct {
	Url       string
	Auth      bool
	WebMethod string `json:"method"`
}
type methodStruct struct {
	methodName       string
	methodProperties methodProperties
	structToValidate string
}

func main() {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, os.Args[1], nil, parser.ParseComments)
	if err != nil {
		log.Fatal(err)
	}

	out, _ := os.Create(os.Args[2])
	fmt.Fprintln(out, `package `+node.Name.Name)
	fmt.Fprintln(out, `
import (
	"net/http"
	"fmt"
	"errors"
	"strconv"
	"encoding/json"
)`)

	regJSON := regexp.MustCompile(`\{".+\:.*\}`)

	//collect methods
	methods := map[string][]methodStruct{}
	structsString := map[string]struct{}{}
	for _, n := range node.Decls {
		function, ok := n.(*ast.FuncDecl)
		if !ok {
			fmt.Printf("SKIP %T is not *ast.FuncDecl\n", n)
			continue
		}
		if function.Doc == nil {
			fmt.Printf("SKIP func %#v doesnt have comments\n", function.Name.Name)
			continue
		}
		needCodegen := false
		methodProperties := methodProperties{}
		for _, comment := range function.Doc.List {
			needCodegen = needCodegen || strings.HasPrefix(comment.Text, "// apigen:api")
			if needCodegen {
				jsn := regJSON.FindString(comment.Text)
				if jsn != "" {
					err := json.Unmarshal([]byte(jsn), &methodProperties)
					if err != nil {
						fmt.Printf("SKIP comment %s for func %s couln't Unmarhsal: %s\n",
							comment.Text, function.Name.Name, err.Error())
						continue
					}
				}
			}
		}
		if !needCodegen || function.Recv == nil {
			fmt.Printf("SKIP struct %#v doesnt have apigen:api mark\n", function.Name.Name)
			continue
		}

		// collect info about methods:
		// structName
		//		methodName
		//		methodProperties
		var structName string
		if structNameExpr, ok := function.Recv.List[0].Type.(*ast.StarExpr); ok {
			val, ok := structNameExpr.X.(*ast.Ident)
			if !ok {
				continue
			}
			structName = val.Name
		} else {
			continue
		}
		methods[structName] = append(methods[structName], methodStruct{
			methodName:       function.Name.String(),
			methodProperties: methodProperties, // URL, METHOD (POST), AUTH
			structToValidate: function.Type.Params.List[1].Type.(*ast.Ident).String(),
		})
		structsString[function.Type.Params.List[1].Type.(*ast.Ident).String()] = struct{}{}

		// return values
		//function.Type.Results.
	}

	//collect struct
	structs := map[string]*ast.StructType{}
	for _, n := range node.Decls {
		g, ok := n.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range g.Specs {
			currType, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			structType, ok := currType.Type.(*ast.StructType)
			if !ok {
				continue
			}
			if _, ok := structsString[currType.Name.Name]; ok {
				structs[currType.Name.Name] = structType
			}
		}
	}

	//write ServeHTTP methods
	for structName, method := range methods {
		fmt.Fprintln(out, `
func (h *`+structName+`) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {`)
		for _, m := range method {
			fmt.Fprintln(out, `	case "`+m.methodProperties.Url+`":
		h.Wrapper`+m.methodName+`(w, r)`)
		}
		fmt.Fprintln(out, `	default:
		http.Error(w, `+"`"+`{"error": "unknown method"}`+"`"+`, http.StatusNotFound)
	}
}`)
	}

	//write WrapperFuncs
	for structName, method := range methods {
		for _, m := range method {
			fmt.Println(m.methodProperties)
			fmt.Fprintln(out, `
func (h *`+structName+`) Wrapper`+m.methodName+`(w http.ResponseWriter, r *http.Request) {`)
			if m.methodProperties.WebMethod == "POST" {
				fmt.Fprintln(out, `	if r.Method != "`+m.methodProperties.WebMethod+`" {
		http.Error(w, `+"`"+`{"error": "bad method"}`+"`"+`, http.StatusNotAcceptable)
		return
	}`)
			}
			if m.methodProperties.Auth {
				fmt.Fprintln(out, `	if r.Header.Get("X-Auth") != "100500" {
		http.Error(w, `+"`"+`{"error": "unauthorized"}`+"`"+`, http.StatusForbidden)
		return
	}`)
			}

			//unpack data
			fmt.Fprintln(out, `
	params := `+m.structToValidate+`{}
	if err := params.Unpack(r); err != nil {
		errStruct := err.(ApiError)
		errString := `+"`"+`{"error": "`+"`"+`+errStruct.Error()+`+"`"+`"}`+"`"+`
		http.Error(w, errString, errStruct.HTTPStatus)
		return
	}`)

			fmt.Fprintln(out, `
	res, err := h.`+m.methodName+`(r.Context(), params)
	if err != nil {
		if errStruct, ok := err.(ApiError); ok {
			errString := `+"`"+`{"error": "`+"`"+`+errStruct.Error()+`+"`"+`"}`+"`"+`
			http.Error(w, errString, errStruct.HTTPStatus)
		} else {
			errString := `+"`"+`{"error": "`+"`"+`+err.Error()+`+"`"+`"}`+"`"+`
			http.Error(w, errString, http.StatusInternalServerError)
		}
		return
	}

	reply := map[string]interface{}{}
	reply["error"] = ""
	reply["response"] = res	
	result, err := json.Marshal(reply); if err != nil {
		errString := `+"`"+`{"error": "cant pack json"}`+"`"+`
		fmt.Println(errString)
		http.Error(w, errString, http.StatusInternalServerError)
		return
	}
	w.Write(result)
}`)
		}
	}

	//write Unpack
	for _, method := range methods {
		for _, m := range method {
			fmt.Println(m.methodProperties)
			fmt.Fprintln(out, `
func (params *`+m.structToValidate+`) Unpack(r *http.Request) error {`)
			for _, field := range structs[m.structToValidate].Fields.List {
				var tagValue string
				if field.Tag != nil {
					tag := reflect.StructTag(field.Tag.Value[1 : len(field.Tag.Value)-1])
					tagValue = tag.Get("apivalidator")
				}

				fieldName := field.Names[0].Name
				fileType := field.Type.(*ast.Ident).Name
				if fileType != "int" && fileType != "string" {
					panic(errors.New("apivalidator: sorry, don't support type: "+fileType))
				}

				tags, err := tagsToMap(strings.Split(tagValue, ","))
				if err != nil {
					panic(errors.New("some error during tags unpack" + err.Error()))
				}

				if tags != nil {
					field := strings.ToLower(fieldName)
					if val, ok := tags["paramname"]; ok {
						field = val.(string)
					}
					fmt.Fprintln(out, `
	if `+fieldName+`Val := r.FormValue("`+field+`"); `+fieldName+`Val != "" {`)

					if fileType == "int" {
						fmt.Fprintln(out, `
		num`+fieldName+`, err := strconv.Atoi(`+fieldName+`Val); if err != nil {
			return ApiError{
				HTTPStatus: http.StatusBadRequest,
				Err:        errors.New(`+"`"+field+` must be int`+"`"+`),
			}
		}`)
						if val, ok := tags["enum"]; ok {
							enums := val.([]string)
							fmt.Fprint(out, `		if `+fieldName+`Val != `)
							enumss := "["
							for i, enum := range enums {
								fmt.Fprint(out, enum)
								if len(enums)-1 != i {
									fmt.Fprintln(out, ` &&`)
									fmt.Fprint(out, `		`+fieldName+`Val != `)
									enumss += enum+", "
								} else {
									enumss += enum+"]"
								}
							}
							fmt.Fprint(out, `{`)
							fmt.Fprintln(out, `
			return ApiError{
				HTTPStatus: http.StatusBadRequest,
				Err:        errors.New(`+"`"+field+` must be one of `+enumss+"`"+`),
			}
		}`)
						}
						if val, ok := tags["max"]; ok {
							num := val.(int)
							numStr := strconv.Itoa(num)
							fmt.Fprintln(out, `
		if num`+fieldName+` > `+numStr+` {
			return ApiError{
				HTTPStatus: http.StatusBadRequest,
				Err:        errors.New(`+"`"+field+` must be <= `+numStr+"`"+`),
			}
		}`)
						}
						if val, ok := tags["min"]; ok {
							num := val.(int)
							numStr := strconv.Itoa(num)
							fmt.Fprintln(out, `
		if num`+fieldName+` < `+numStr+` {
			return ApiError{
				HTTPStatus: http.StatusBadRequest,
				Err:        errors.New(`+"`"+field+` must be >= `+numStr+"`"+`),
			}
		}`)
						}
						fmt.Fprint(out, `
		params.`+fieldName+` = num`+fieldName+`
	}`)
					} else {
						if val, ok := tags["enum"]; ok {
							enums := val.([]string)
							fmt.Fprint(out, `		if `+fieldName+`Val != "`)
							enumss := "["
							for i, enum := range enums {
								fmt.Fprint(out, enum+`"`)
								if len(enums)-1 != i {
									fmt.Fprintln(out, ` &&`)
									fmt.Fprint(out, `		`+fieldName+`Val != "`)
									enumss += enum + ", "
								} else {
									enumss += enum + "]"
								}
							}
							fmt.Fprint(out, `{`)
							fmt.Fprintln(out, `
			return ApiError{
				HTTPStatus: http.StatusBadRequest,
				Err:        errors.New(`+"`"+field+` must be one of `+enumss+"`"+`),
			}
		}`)
						}
						if val, ok := tags["max"]; ok {
							num := val.(int)
							numStr := strconv.Itoa(num)
							fmt.Fprintln(out, `
	if len(`+fieldName+`Val) > `+numStr+` {
		return ApiError{
			HTTPStatus: http.StatusBadRequest,
			Err:        errors.New(`+"`"+field+` len must be <= `+numStr+"`"+`),
		}
	}`)
						}
						if val, ok := tags["min"]; ok {
							num := val.(int)
							numStr := strconv.Itoa(num)
							fmt.Fprintln(out, `
	if len(`+fieldName+`Val) < `+numStr+` {
		return ApiError{
			HTTPStatus: http.StatusBadRequest,
			Err:        errors.New(`+"`"+field+` len must be >= `+numStr+"`"+`),
		}
	}`)
						}

						fmt.Fprint(out, `
		params.`+fieldName+` = `+fieldName+`Val
	}`)
					}

					if _, ok := tags["required"]; ok {
						fmt.Fprintln(out, ` else {
		return ApiError{
			HTTPStatus: http.StatusBadRequest,
			Err:        errors.New(`+"`"+field+` must me not empty`+"`"+`),
		}
	}`)
					}
					if val, ok := tags["default"]; ok {
						value := val.(string)
						if fileType == "int" {
							fmt.Fprintln(out, ` else {
		params.`+fieldName+` = `+value+`
	}`)
						} else {
							fmt.Fprintln(out, ` else {
		params.`+fieldName+` = "`+value+`"
	}`)
						}
					}
				} else { // if tags are empty
					field := strings.ToLower(fieldName)
					fmt.Fprintln(out, `
	if `+fieldName+`Val := r.FormValue("`+field+`"); `+fieldName+`Val != "" {`)
					if fileType == "int" {
							fmt.Fprintln(out, `
		num`+fieldName+`, err := strconv.Atoi(`+fieldName+`Val); if err != nil {
			return ApiError{
				HTTPStatus: http.StatusBadRequest,
				Err:        errors.New(`+"`"+field+` must be int`+"`"+`),
			}
		}
		params.`+fieldName+` = num`+fieldName+`
`)
						} else {
						fmt.Fprintln(out, `		params.`+fieldName+` = `+fieldName+`Val`)
					}
				}

			}
			fmt.Fprintln(out, `
	return nil
}`)
		}
	}
}

func tagsToMap(tags []string) (map[string]interface{}, error) {
	if tags == nil {
		return nil, nil
	}
	result := map[string]interface{}{}
	for _, tag := range tags {
		if strings.Contains(tag, "paramname=") {
			val, err := getValue(tag, 10, "")
			if err != nil {
				return nil, err
			}
			result["paramname"] = val
		}
		if tag == "required" {
			result["required"] = ""
		}
		if strings.Contains(tag, "enum=") {
			val, err := getValue(tag, 5, "slice")
			if err != nil {
				return nil, err
			}
			result["enum"] = val
		}
		if strings.Contains(tag, "default=") {
			val, err := getValue(tag, 8, "")
			if err != nil {
				return nil, err
			}
			result["default"] = val
		}
		if strings.Contains(tag, "max=") {
			val, err := getValue(tag, 4, "int")
			if err != nil {
				return nil, err
			}
			result["max"] = val
		}
		if strings.Contains(tag, "min=") {
			val, err := getValue(tag, 4, "int")
			if err != nil {
				return nil, err
			}
			result["min"] = val
		}
	}
	return result, nil
}

func getValue(str string, key int, typ string) (interface{}, error) {
	if key >= len(str) {
		return nil, errors.New("coulnd't parse field in tag: " + str + "\nat byte" + strconv.Itoa(key))
	}
	value := make([]byte, 0, 1)
	for i := key; i < len(str); i++ {
		value = append(value, str[i])
	}
	if typ == "slice" {
		return strings.Split(string(value), "|"), nil
	} else if typ == "int" {
		num, err := strconv.Atoi(string(value))
		if err != nil {
			return nil, errors.New("coulnd't atoi field in tag: " + str + "\nat byte" + strconv.Itoa(key) + " atoi: " + err.Error())
		}
		return num, nil
	}
	return string(value), nil
}

// go build handlers_gen/* && ./codegen api.go apiWrapper.go
