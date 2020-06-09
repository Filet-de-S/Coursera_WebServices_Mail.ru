package main

import (
	"net/http"
	"fmt"
	"errors"
	"strconv"
	"encoding/json"
)

func (h *MyApi) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/user/profile":
		h.WrapperProfile(w, r)
	case "/user/create":
		h.WrapperCreate(w, r)
	default:
		http.Error(w, `{"error": "unknown method"}`, http.StatusNotFound)
	}
}

func (h *OtherApi) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/user/create":
		h.WrapperCreate(w, r)
	default:
		http.Error(w, `{"error": "unknown method"}`, http.StatusNotFound)
	}
}

func (h *MyApi) WrapperProfile(w http.ResponseWriter, r *http.Request) {

	params := ProfileParams{}
	if err := params.Unpack(r); err != nil {
		errStruct := err.(ApiError)
		errString := `{"error": "`+errStruct.Error()+`"}`
		http.Error(w, errString, errStruct.HTTPStatus)
		return
	}

	res, err := h.Profile(r.Context(), params)
	if err != nil {
		if errStruct, ok := err.(ApiError); ok {
			errString := `{"error": "`+errStruct.Error()+`"}`
			http.Error(w, errString, errStruct.HTTPStatus)
		} else {
			errString := `{"error": "`+err.Error()+`"}`
			http.Error(w, errString, http.StatusInternalServerError)
		}
		return
	}

	reply := map[string]interface{}{}
	reply["error"] = ""
	reply["response"] = res	
	result, err := json.Marshal(reply); if err != nil {
		errString := `{"error": "cant pack json"}`
		fmt.Println(errString)
		http.Error(w, errString, http.StatusInternalServerError)
		return
	}
	w.Write(result)
}

func (h *MyApi) WrapperCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, `{"error": "bad method"}`, http.StatusNotAcceptable)
		return
	}
	if r.Header.Get("X-Auth") != "100500" {
		http.Error(w, `{"error": "unauthorized"}`, http.StatusForbidden)
		return
	}

	params := CreateParams{}
	if err := params.Unpack(r); err != nil {
		errStruct := err.(ApiError)
		errString := `{"error": "`+errStruct.Error()+`"}`
		http.Error(w, errString, errStruct.HTTPStatus)
		return
	}

	res, err := h.Create(r.Context(), params)
	if err != nil {
		if errStruct, ok := err.(ApiError); ok {
			errString := `{"error": "`+errStruct.Error()+`"}`
			http.Error(w, errString, errStruct.HTTPStatus)
		} else {
			errString := `{"error": "`+err.Error()+`"}`
			http.Error(w, errString, http.StatusInternalServerError)
		}
		return
	}

	reply := map[string]interface{}{}
	reply["error"] = ""
	reply["response"] = res	
	result, err := json.Marshal(reply); if err != nil {
		errString := `{"error": "cant pack json"}`
		fmt.Println(errString)
		http.Error(w, errString, http.StatusInternalServerError)
		return
	}
	w.Write(result)
}

func (h *OtherApi) WrapperCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, `{"error": "bad method"}`, http.StatusNotAcceptable)
		return
	}
	if r.Header.Get("X-Auth") != "100500" {
		http.Error(w, `{"error": "unauthorized"}`, http.StatusForbidden)
		return
	}

	params := OtherCreateParams{}
	if err := params.Unpack(r); err != nil {
		errStruct := err.(ApiError)
		errString := `{"error": "`+errStruct.Error()+`"}`
		http.Error(w, errString, errStruct.HTTPStatus)
		return
	}

	res, err := h.Create(r.Context(), params)
	if err != nil {
		if errStruct, ok := err.(ApiError); ok {
			errString := `{"error": "`+errStruct.Error()+`"}`
			http.Error(w, errString, errStruct.HTTPStatus)
		} else {
			errString := `{"error": "`+err.Error()+`"}`
			http.Error(w, errString, http.StatusInternalServerError)
		}
		return
	}

	reply := map[string]interface{}{}
	reply["error"] = ""
	reply["response"] = res	
	result, err := json.Marshal(reply); if err != nil {
		errString := `{"error": "cant pack json"}`
		fmt.Println(errString)
		http.Error(w, errString, http.StatusInternalServerError)
		return
	}
	w.Write(result)
}

func (params *ProfileParams) Unpack(r *http.Request) error {

	if LoginVal := r.FormValue("login"); LoginVal != "" {

		params.Login = LoginVal
	} else {
		return ApiError{
			HTTPStatus: http.StatusBadRequest,
			Err:        errors.New(`login must me not empty`),
		}
	}

	return nil
}

func (params *CreateParams) Unpack(r *http.Request) error {

	if LoginVal := r.FormValue("login"); LoginVal != "" {

	if len(LoginVal) < 10 {
		return ApiError{
			HTTPStatus: http.StatusBadRequest,
			Err:        errors.New(`login len must be >= 10`),
		}
	}

		params.Login = LoginVal
	} else {
		return ApiError{
			HTTPStatus: http.StatusBadRequest,
			Err:        errors.New(`login must me not empty`),
		}
	}

	if NameVal := r.FormValue("full_name"); NameVal != "" {

		params.Name = NameVal
	}
	if StatusVal := r.FormValue("status"); StatusVal != "" {
		if StatusVal != "user" &&
		StatusVal != "moderator" &&
		StatusVal != "admin"{
			return ApiError{
				HTTPStatus: http.StatusBadRequest,
				Err:        errors.New(`status must be one of [user, moderator, admin]`),
			}
		}

		params.Status = StatusVal
	} else {
		params.Status = "user"
	}

	if AgeVal := r.FormValue("age"); AgeVal != "" {

		numAge, err := strconv.Atoi(AgeVal); if err != nil {
			return ApiError{
				HTTPStatus: http.StatusBadRequest,
				Err:        errors.New(`age must be int`),
			}
		}

		if numAge > 128 {
			return ApiError{
				HTTPStatus: http.StatusBadRequest,
				Err:        errors.New(`age must be <= 128`),
			}
		}

		if numAge < 0 {
			return ApiError{
				HTTPStatus: http.StatusBadRequest,
				Err:        errors.New(`age must be >= 0`),
			}
		}

		params.Age = numAge
	}
	return nil
}

func (params *OtherCreateParams) Unpack(r *http.Request) error {

	if UsernameVal := r.FormValue("username"); UsernameVal != "" {

	if len(UsernameVal) < 3 {
		return ApiError{
			HTTPStatus: http.StatusBadRequest,
			Err:        errors.New(`username len must be >= 3`),
		}
	}

		params.Username = UsernameVal
	} else {
		return ApiError{
			HTTPStatus: http.StatusBadRequest,
			Err:        errors.New(`username must me not empty`),
		}
	}

	if NameVal := r.FormValue("account_name"); NameVal != "" {

		params.Name = NameVal
	}
	if ClassVal := r.FormValue("class"); ClassVal != "" {
		if ClassVal != "warrior" &&
		ClassVal != "sorcerer" &&
		ClassVal != "rouge"{
			return ApiError{
				HTTPStatus: http.StatusBadRequest,
				Err:        errors.New(`class must be one of [warrior, sorcerer, rouge]`),
			}
		}

		params.Class = ClassVal
	} else {
		params.Class = "warrior"
	}

	if LevelVal := r.FormValue("level"); LevelVal != "" {

		numLevel, err := strconv.Atoi(LevelVal); if err != nil {
			return ApiError{
				HTTPStatus: http.StatusBadRequest,
				Err:        errors.New(`level must be int`),
			}
		}

		if numLevel > 50 {
			return ApiError{
				HTTPStatus: http.StatusBadRequest,
				Err:        errors.New(`level must be <= 50`),
			}
		}

		if numLevel < 1 {
			return ApiError{
				HTTPStatus: http.StatusBadRequest,
				Err:        errors.New(`level must be >= 1`),
			}
		}

		params.Level = numLevel
	}
	return nil
}
