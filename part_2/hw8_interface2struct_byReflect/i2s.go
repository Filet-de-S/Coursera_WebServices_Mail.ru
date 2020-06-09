package main

import (
	"errors"
	"fmt"
	"reflect"
)

func i2s(data interface{}, out interface{}) error {
	vOut := reflect.ValueOf(out)
	if vOut.Kind() != reflect.Ptr {
		return errors.New("not pointer: " + vOut.Kind().String())
	}
	vOut = vOut.Elem()
	dataType := vOut.Type()

	switch dataType.Kind() {
	case reflect.Struct:
		{
			mapData, ok := data.(map[string]interface{})
			if !ok {
				return errors.New("input data is not map[string]interface{}, but: " + reflect.TypeOf(data).String())
			}

			for i := 0; i < vOut.NumField(); i++ {
				f := vOut.Field(i)
				if !f.CanSet() {
					fmt.Println("!can't Set", f)
					continue
				}
				if v, ok := mapData[dataType.Field(i).Name]; ok {
					if err := insertToField(v, f); err != nil {
						return err
					}
				}
			}
		}
	case reflect.Slice:
		{
			sliceData, ok := data.([]interface{})
			if !ok {
				return errors.New("input data is not slice[]interface{}, but: " + reflect.TypeOf(data).String())
			}

			for i := range sliceData {
				newVal := reflect.New(vOut.Type().Elem())
				if err := i2s(sliceData[i], newVal.Interface()); err != nil {
					return err
				}
				vOut.Set(reflect.Append(vOut, newVal.Elem()))
			}
		}
	default:
		return errors.New("doesn't support that type: " + dataType.Kind().String())
	}

	return nil
}

func insertToField(v interface{}, f reflect.Value) error {
	inputValue := reflect.ValueOf(v)

	if f.Type() == inputValue.Type() {
		f.Set(inputValue)
	} else if f.Kind() == inputValue.Kind() {
		if err := i2s(inputValue.Interface(), f.Addr().Interface()); err != nil {
			return err
		}
	} else if f.Kind() == reflect.Int && inputValue.Kind() == reflect.Float64 {
		intV := int64(inputValue.Float())
		f.SetInt(intV)
	} else if f.Kind() == reflect.Struct && inputValue.Kind() == reflect.Map {
		if !f.CanAddr() {
			return errors.New("can take addr of field")
		}

		if err := i2s(v, f.Addr().Interface()); err != nil {
			return err
		}
	} else {
		return errors.New("distinct types: input is " +
			inputValue.Kind().String() +
			" and structField is " + f.Kind().String())
	}

	return nil
}
