package main

import (
	"encoding/json"
	"github.com/mailru/easyjson"
	"github.com/mailru/easyjson/jlexer"
	"github.com/mailru/easyjson/jwriter"
	"io"
	"os"
	"strconv"
	"strings"
)

// вам надо написать более быструю оптимальную этой функции
type Read struct {
	Reader   *os.File
	Buffer   []byte
	Leftover []byte
}

type UserStruct struct {
	Browsers []string `json:"browsers"`
	Email    string   `json:"email"`
	Name     string   `json:"name"`
}

const BUFFSIZE = 25000

func FastSearch(out io.Writer) {
	file, err := os.Open(filePath)
	if err != nil {
		panic(err)
	}

	//user := &easy.UserStruct{}
	user := &UserStruct{}

	seenBrowsers := make(map[string]struct{}, 1000)
	uniqueBrowsers := 0
	isAndroid := false
	isMSIE := false

	out.Write([]byte("found users:\n"))
	i := 0
	buff := &Read{file, make([]byte, BUFFSIZE), []byte{}}
	for {
		textInBytes, err := buff.Readline()
		if textInBytes == nil || err != nil {
			break
		}
		//err = json.Unmarshal(*textInBytes, user)
		err = user.UnmarshalJSON(*textInBytes) // easyJSON
		if err != nil {
			panic(err)
			return
		}

		isAndroid = false
		isMSIE = false
		for i := range user.Browsers {
			if strings.Contains(user.Browsers[i], "Android") {
				isAndroid = true
				if _, ok := seenBrowsers[user.Browsers[i]]; !ok {
					seenBrowsers[user.Browsers[i]] = struct{}{}
					uniqueBrowsers++
				}
			}

			if strings.Contains(user.Browsers[i], "MSIE") {
				isMSIE = true
				if _, ok := seenBrowsers[user.Browsers[i]]; !ok {
					seenBrowsers[user.Browsers[i]] = struct{}{}
					uniqueBrowsers++
				}
			}
		}

		if !(isAndroid && isMSIE) {
			i++
			continue
		}
		email := replaceAllString(&user.Email)
		out.Write([]byte("["+strconv.Itoa(i)+"] "+user.Name+" <"+*email+">\n"))
		i++

	}
	// todo 559910 B/op 10422 allocs/op
	out.Write([]byte("\nTotal unique browsers "+strconv.Itoa(len(seenBrowsers))+"\n"))
}

func (r *Read) Readline() (*[]byte, error) {
	if r.Leftover != nil {
		for i := range r.Leftover {
			if (r.Leftover)[i] == '\n' {
				toReturn := (r.Leftover)[:i]
				r.Leftover = (r.Leftover)[i+1:]
				return &toReturn, nil
			}
		}
		dst := make([]byte, len(r.Leftover))
		copy(dst, r.Leftover)
		r.Leftover = dst
	}

	for {
		n, err := r.Reader.Read(r.Buffer)
		if err != nil && n == 0 && r.Leftover == nil {
			return nil, err
		} else if err != nil && n == 0 {
			toReturn := r.Leftover
			r.Leftover = nil
			return &toReturn, nil
		}

		for i := 0; i < n; i++ {
			if (r.Buffer)[i] == '\n' {
				toReturn := append(r.Leftover, (r.Buffer)[:i]...)
				r.Leftover = (r.Buffer)[i+1 : n]
				return &toReturn, nil
			}
		}
		r.Leftover = append(r.Leftover, r.Buffer[:n]...)
		//dst := make([]byte, len(r.Leftover))
		//copy(dst, r.Leftover)
		//r.Leftover = dst
	} //end for
}

var at = []byte(" [at] ")

func replaceAllString(s *string) *string {
	str := []byte(*s)
	for i := range str {
		if str[i] == '@' {
			str = append(str[:i], append(at, str[i+1:]...)...)
		}
	}
	st := string(str)
	return &st
}


// suppress unused package warning
var (
	_ *json.RawMessage
	_ *jlexer.Lexer
	_ *jwriter.Writer
	_ easyjson.Marshaler
)

func easyjson97766e5aDecodeMyProjectsGoMailruPart1Week3Hw3BenchEasy(in *jlexer.Lexer, out *UserStruct) {
	isTopLevel := in.IsStart()
	if in.IsNull() {
		if isTopLevel {
			in.Consumed()
		}
		in.Skip()
		return
	}
	in.Delim('{')
	for !in.IsDelim('}') {
		key := in.UnsafeFieldName(false)
		in.WantColon()
		if in.IsNull() {
			in.Skip()
			in.WantComma()
			continue
		}
		switch key {
		case "browsers":
			if in.IsNull() {
				in.Skip()
				out.Browsers = nil
			} else {
				in.Delim('[')
				if out.Browsers == nil {
					if !in.IsDelim(']') {
						out.Browsers = make([]string, 0, 4)
					} else {
						out.Browsers = []string{}
					}
				} else {
					out.Browsers = (out.Browsers)[:0]
				}
				for !in.IsDelim(']') {
					var v1 string
					v1 = string(in.String())
					out.Browsers = append(out.Browsers, v1)
					in.WantComma()
				}
				in.Delim(']')
			}
		case "email":
			out.Email = string(in.String())
		case "name":
			out.Name = string(in.String())
		default:
			in.SkipRecursive()
		}
		in.WantComma()
	}
	in.Delim('}')
	if isTopLevel {
		in.Consumed()
	}
}
func easyjson97766e5aEncodeMyProjectsGoMailruPart1Week3Hw3BenchEasy(out *jwriter.Writer, in UserStruct) {
	out.RawByte('{')
	first := true
	_ = first
	{
		const prefix string = ",\"browsers\":"
		out.RawString(prefix[1:])
		if in.Browsers == nil && (out.Flags&jwriter.NilSliceAsEmpty) == 0 {
			out.RawString("null")
		} else {
			out.RawByte('[')
			for v2, v3 := range in.Browsers {
				if v2 > 0 {
					out.RawByte(',')
				}
				out.String(string(v3))
			}
			out.RawByte(']')
		}
	}
	{
		const prefix string = ",\"email\":"
		out.RawString(prefix)
		out.String(string(in.Email))
	}
	{
		const prefix string = ",\"name\":"
		out.RawString(prefix)
		out.String(string(in.Name))
	}
	out.RawByte('}')
}

// MarshalJSON supports json.Marshaler interface
func (v UserStruct) MarshalJSON() ([]byte, error) {
	w := jwriter.Writer{}
	easyjson97766e5aEncodeMyProjectsGoMailruPart1Week3Hw3BenchEasy(&w, v)
	return w.Buffer.BuildBytes(), w.Error
}

// MarshalEasyJSON supports easyjson.Marshaler interface
func (v UserStruct) MarshalEasyJSON(w *jwriter.Writer) {
	easyjson97766e5aEncodeMyProjectsGoMailruPart1Week3Hw3BenchEasy(w, v)
}

// UnmarshalJSON supports json.Unmarshaler interface
func (v *UserStruct) UnmarshalJSON(data []byte) error {
	r := jlexer.Lexer{Data: data}
	easyjson97766e5aDecodeMyProjectsGoMailruPart1Week3Hw3BenchEasy(&r, v)
	return r.Error()
}

// UnmarshalEasyJSON supports easyjson.Unmarshaler interface
func (v *UserStruct) UnmarshalEasyJSON(l *jlexer.Lexer) {
	easyjson97766e5aDecodeMyProjectsGoMailruPart1Week3Hw3BenchEasy(l, v)
}
