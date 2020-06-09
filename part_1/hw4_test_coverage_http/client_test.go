package main

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"testing"
	"time"
)

type UserRow struct {
	Id     int    `xml:"id"`
	FName  string `xml:"first_name"`
	LName  string `xml:"last_name"`
	Age    int    `xml:"age"`
	About  string `xml:"about"`
	Gender string `xml:"gender"`
}

func getParsedData(order int, field string) (*[]User, error) {
	type Users struct {
		List []UserRow `xml:"row"`
	}

	data, err := ioutil.ReadFile("dataset.xml")
	if err != nil {
		return nil, err
	}

	v := Users{}
	err = xml.Unmarshal(data, &v)
	if err != nil {
		return nil, err
	}

	typedUsers := make([]User, 0, len(v.List))
	for _, u := range v.List {
		typedUsers = append(typedUsers, User{
			Id:     u.Id,
			Name:   u.FName + u.LName,
			Age:    u.Age,
			About:  u.About,
			Gender: u.Gender,
		})
	}


	if order == orderAsc {
		switch field {
		case "Name":
			sort.Slice(typedUsers, func(i, j int) bool {
				return typedUsers[i].Name < typedUsers[j].Name })
		case "Age":
			sort.Slice(typedUsers, func(i, j int) bool {
				return typedUsers[i].Age < typedUsers[j].Age })
		case "Id":
			sort.Slice(typedUsers, func(i, j int) bool {
				return typedUsers[i].Id < typedUsers[j].Id })
		}
	} else if order == orderDesc {
		switch field {
		case "Name":
			sort.Slice(typedUsers, func(i, j int) bool {
				return typedUsers[i].Name > typedUsers[j].Name })
		case "Age":
			sort.Slice(typedUsers, func(i, j int) bool {
				return typedUsers[i].Age > typedUsers[j].Age })
		case "Id":
			sort.Slice(typedUsers, func(i, j int) bool {
				return typedUsers[i].Id > typedUsers[j].Id })
		}
	}

	return &typedUsers, nil
}

func SearchServer(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("AccessToken") != "ok" {
		http.Error(w, "Bad AccessToken", http.StatusUnauthorized)
		return
	}

	sr, err := getSearchReq(r); if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if sr.OrderBy == 4 || (sr.OrderBy != OrderByAsc &&
		sr.OrderBy != OrderByAsIs && sr.OrderBy != OrderByDesc) {
		http.Error(w, "SearchServer fatal error", http.StatusInternalServerError)
		return
	} else if sr.OrderBy == 1 {
		sr.OrderBy = orderAsc
	} else {
		sr.OrderBy = orderDesc
	}

	data, err := getParsedData(sr.OrderBy, sr.OrderField)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if sr.Query == "" {
		data, err := json.Marshal(*data); if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		time.Sleep(time.Second)
		w.Write(data)
		return
	}
	if sr.Offset > 25 {
		w.Write([]byte(`cad`))
		return
	}

	res := []User{}
	for i := sr.Offset; i < len(*data); i++ {
		if len(res) > sr.Limit {
			break
		}
		if (*data)[i].Name == sr.Query || (*data)[i].About == sr.Query {
			res = append(res, (*data)[i])
		}
	}
	dataToReturn, err := json.Marshal(res); if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(dataToReturn)
}

func getSearchReq(r *http.Request) (*SearchRequest, error) {
	s := &SearchRequest{}
	var err error

	if lim := r.FormValue("limit"); lim != "" {
		s.Limit, err = strconv.Atoi(lim)
		if err != nil {
			return nil, err
		}
	}
	if offset := r.FormValue("offset"); offset != "" {
		s.Offset, err = strconv.Atoi(offset)
		if err != nil {
			return nil, err
		}
	}
	if orderBy := r.FormValue("order_by"); orderBy != "" {
		s.OrderBy, err = strconv.Atoi(orderBy)
		if err != nil {
			return nil, err
		}
		if s.OrderBy == 2 {
			resp := SearchErrorResponse{Error: "ErrorBadOrderField"}
			data, err := json.Marshal(resp)
			if err != nil {
				return nil, err
			}
			return nil, errors.New(string(data))
		}
		if s.OrderBy == 3 {
			return nil, errors.New(`string(data)`)
		}
	}

	s.Query = r.FormValue("query")
	s.OrderField = r.FormValue("order_field")
	if s.OrderField != "" && s.OrderField != "Id" && s.OrderField != "Age" &&
		s.OrderField != "Name" {
		data, err := json.Marshal(SearchErrorResponse{Error: ErrorBadOrderField})
		if err != nil {
			return nil, err
		}
		return nil, errors.New(string(data))
	}
	if s.OrderField == "" {
		s.OrderField = "Name"
	}
	return s, nil
}

type testCase struct {
	request  SearchRequest
	response SearchResponse
}

func TestOK(t *testing.T) {
	searchClient := newSearchClient()
	v, err := searchClient.FindUsers(SearchRequest{Query: "DicksonSilva"})
	if err != nil {
		t.Fatal(err)
	}
	if !v.NextPage {
		t.Fatal(v.Users)
	}

	v, _ = searchClient.FindUsers(SearchRequest{Query: "DicksonSilva", Limit: 26})
	if v.Users[0].Name != "DicksonSilva" || v.NextPage {
		t.Fatal()
	}
}

func TestTimeoutAndUnknown(t *testing.T) {
	searchClient := newSearchClient()
	_, err := searchClient.FindUsers(SearchRequest{})
	if err == nil {
		t.Fatal()
	}

	searchClient.URL = "neUrl"
	_, err = searchClient.FindUsers(SearchRequest{OrderBy: 4})
	if err == nil {
		t.Fatal()
	}
}

func TestLimitOffset(t *testing.T) {
	searchClient := newSearchClient()
	req := SearchRequest{Limit: -1}
	_, err := searchClient.FindUsers(req)
	if err == nil ||
		err.Error() != "limit must be > 0" {
		t.Fatal(err)
	}

	req = SearchRequest{Offset: -1}
	_, err = searchClient.FindUsers(req)
	if err == nil ||
		err.Error() != "offset must be > 0" {
		t.Fatal(err)
	}
}

func TestStatusCode(t *testing.T) {
	cases := []testCase{
		testCase{request: SearchRequest{OrderBy: 2}},
		testCase{request: SearchRequest{OrderBy: 3}},
		testCase{request: SearchRequest{Query: "hue", Offset: 26}},
		testCase{request: SearchRequest{OrderField: "NameS"}},
	}

	searchClient := newSearchClient()
	for i := range cases {
		_, err := searchClient.FindUsers(cases[i].request)
		if err == nil {
			t.Fatal()
		}
	}

	//
	_, err := searchClient.FindUsers(SearchRequest{OrderBy: 4})
	if err == nil || err.Error() != "SearchServer fatal error" {
		t.Fatal(err)
	}

	////
	searchClient.AccessToken = "ohoh"
	_, err = searchClient.FindUsers(SearchRequest{})
	if err == nil || err.Error() != "Bad AccessToken" {
		t.Fatal()
	}
}

func newSearchClient() SearchClient {
	ts := httptest.NewServer(http.HandlerFunc(SearchServer))
	return SearchClient{
		AccessToken: "ok",
		URL:         ts.URL,
	}
}
