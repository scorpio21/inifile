package inifile

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
)

/*
*
* Easy INI File manager for Golang
* Jonathan G. Hecl
* https://github.com/jonathanhecl/inifile
*
 */

type _EType int8

const (
	SECTION _EType = iota
	KEY
	IGNORED
)

type _TLine struct {
	Mode    _EType
	Section string
	Key     string
	Value   string
	Line    string
}

var _Section []byte = []byte{91, 93}
var _KeyValueDiff byte = byte(61)
var _FlagComments []byte = []byte{35, 39, 47, 96} // 47 double
var _IgnoredSpaces []byte = []byte{9, 10, 32}

type TValue struct {
	Value []byte
}

type TINIFile struct {
	lines      []_TLine
	Filename   string
	TotalLines int
	options    *TOptions
}

type TOptions struct {
	Debug         bool
	CaseSensitive bool
}

func (t *TINIFile) Options(o *TOptions) {
	(*t).options = o
}

func New(o *TOptions) *TINIFile {
	t := TINIFile{}
	t.lines = []_TLine{}
	t.Filename = ""
	t.TotalLines = 0
	t.options = o
	if t.options == nil {
		t.options = &TOptions{
			CaseSensitive: false,
			Debug:         false,
		}
	}
	return &t
}

func Load(Path string, o *TOptions) (*TINIFile, error) {
	t := TINIFile{}
	t.lines = []_TLine{}
	t.Filename = Path
	t.TotalLines = 0
	t.options = o
	if t.options == nil {
		t.options = &TOptions{
			CaseSensitive: false,
			Debug:         false,
		}
	}
	if f, err := os.Open(t.Filename); err != nil {
		return nil, err
	} else {
		defer f.Close()
		s := bufio.NewScanner(f)
		lineNumber := 0
		for s.Scan() {
			l := strings.TrimSpace(s.Text())
			if lineNumber == 0 {
				t.lines = append(t.lines, t.processLine(l, _TLine{}))
			} else {
				t.lines = append(t.lines, t.processLine(l, t.lines[lineNumber-1]))
			}
			lineNumber++
		}
		t.TotalLines = lineNumber
	}
	return &t, nil
}

func (t *TINIFile) Save(Path string) error {
	if f, err := os.Create(Path); err != nil {
		return err
	} else {
		defer f.Close()
		lineBreak := "\r"
		if runtime.GOOS == "windows" {
			lineBreak = "\r\n"
		}
		for i := range t.lines {
			if _, err := f.Write([]byte(t.lines[i].Line + lineBreak)); err != nil {
				panic(err)
			}
		}
	}
	return nil
}

// Logic

func (t *TINIFile) processLine(line string, prevLine _TLine) _TLine {
	r := _TLine{
		Mode:    IGNORED,
		Section: prevLine.Section,
		Line:    line,
	}
	ignoringBeginning := true
	ignoringComment := false
	capturingSection := false
	capturingKey := false
	capturingValue := false
	tempReading := []byte{}
	for i := range line {
		if ignoringBeginning && !bytes.Contains(_IgnoredSpaces, []byte{byte(line[i])}) {
			ignoringBeginning = false
			capturingKey = true
		}
		if !ignoringBeginning {
			if !ignoringComment && bytes.Contains(_FlagComments, []byte{byte(line[i])}) {
				isComment := true
				if byte(line[i]) == 47 && len(line) > i { // 47 special
					if byte(line[i+1]) != 47 {
						isComment = false
					}
				}
				if isComment {
					ignoringComment = true
					capturingKey = false
					break
				}
			}
			if (capturingSection || capturingKey) &&
				!capturingValue && bytes.Contains(_IgnoredSpaces, []byte{byte(line[i])}) {
				capturingSection = false
				capturingKey = false
				break
			}
			if !capturingSection && _Section[0] == byte(line[i]) {
				capturingSection = true
				capturingKey = false
				continue
			} else if capturingSection && _Section[1] == byte(line[i]) {
				r.Mode = SECTION
				r.Section = string(tempReading)
				r.Key = ""
				r.Value = ""
				capturingSection = false
				break
			}
			if capturingKey && _KeyValueDiff == byte(line[i]) {
				r.Mode = KEY
				r.Section = prevLine.Section
				r.Key = string(tempReading)
				r.Value = ""
				tempReading = []byte{}
				capturingValue = true
				continue
			}
			if !ignoringComment {
				tempReading = append(tempReading, byte(line[i]))
				if capturingValue {
					r.Value = strings.TrimSpace(string(tempReading))
				}
			}
		}
	}
	if t.options.Debug {
		fmt.Println("Line analyzed: ", string(line))
		fmt.Println("Line information: ", r)
	}
	return r
}

func (t *TINIFile) Set(section string, key string, value TValue) {
	sectionFound := -1
	for i := range t.lines {
		if (!t.options.CaseSensitive && strings.EqualFold(t.lines[i].Section, section)) ||
			(t.options.CaseSensitive && t.lines[i].Section == section) {
			if t.lines[i].Mode == KEY || sectionFound == -1 {
				sectionFound = i
			}
			if (!t.options.CaseSensitive && strings.EqualFold(t.lines[i].Key, key)) ||
				(t.options.CaseSensitive && t.lines[i].Key == key) {
				if t.options.Debug {
					fmt.Println("EDIT VALUE: ", section, "->", key, "=", string(value.Value))
				}
				key = t.lines[i].Key
				tempKey := []byte(t.lines[i].Line[:strings.Index(t.lines[i].Line, key)+len(key+string(_KeyValueDiff))])
				tempRest := []byte(t.lines[i].Line[len(tempKey):])
				tempNonValue := []byte{}
				if len(t.lines[i].Value) < len(tempRest) {
					tempNonValue = append([]byte{32}, tempRest[len(t.lines[i].Value)+1:]...)

				}
				(*t).lines[i].Value = string(value.Value)
				(*t).lines[i].Line = string(tempKey) + t.lines[i].Value + string(tempNonValue)
				if t.options.Debug {
					fmt.Println("SET RETURN: ", t.lines[i])
				}
				return
			}
		}
	}
	if len(value.Value) > 0 {
		if sectionFound >= 0 {
			if t.options.Debug {
				fmt.Println("NEW KEY: ", section, "->", key, "=", string(value.Value))
			}
			newLine := _TLine{
				Mode:    KEY,
				Section: section,
				Key:     key,
				Value:   string(value.Value),
				Line:    key + string(_KeyValueDiff) + string(value.Value),
			}
			t.lines = append(t.lines, _TLine{})
			copy(t.lines[sectionFound+1:], t.lines[sectionFound:])
			t.lines[sectionFound+1] = newLine
			(*t).lines = t.lines
		} else {
			if t.options.Debug {
				fmt.Println("NEW SECTION: ", section, "->", key, "=", string(value.Value))
			}
			newLines := []_TLine{
				{
					Mode:    SECTION,
					Section: section,
					Line:    string(_Section[0]) + section + string(_Section[1]),
				},
				{
					Mode:    KEY,
					Section: section,
					Key:     key,
					Value:   string(value.Value),
					Line:    key + string(_KeyValueDiff) + string(value.Value),
				},
			}
			t.lines = append(t.lines, newLines...)
			(*t).lines = t.lines
		}
	}
}

func (t *TINIFile) Get(section string, key string) TValue {
	for i := range t.lines {
		if (!t.options.CaseSensitive && strings.EqualFold(t.lines[i].Section, section)) ||
			(t.options.CaseSensitive && t.lines[i].Section == section) {
			if (!t.options.CaseSensitive && strings.EqualFold(t.lines[i].Key, key)) ||
				(t.options.CaseSensitive && t.lines[i].Key == key) {
				return TValue{
					Value: []byte(t.lines[i].Value),
				}
			}
		}
	}
	return TValue{}
}

// Convertions

func String(s string) TValue {
	return TValue{Value: []byte(strings.TrimSpace(s))}
}

func (t TValue) String() string {
	return string(t.Value)
}

func Bool(b bool, isInt bool) TValue {
	s := ""
	if isInt {
		s = "0"
		if b {
			s = "1"
		}
	} else {
		s = "false"
		if b {
			s = "true"
		}
	}
	return TValue{Value: []byte(s)}
}

func (t TValue) Bool() bool {
	b := false
	if strings.EqualFold(string(t.Value), "true") ||
		string(t.Value) == "1" {
		b = true
	}
	return b
}

func Int(i int) TValue {
	s := strconv.Itoa(i)
	return TValue{Value: []byte(s)}
}

func (t TValue) Int() int {
	i, _ := strconv.Atoi(string(t.Value))
	return i
}

func Int8(i int8) TValue {
	s := strconv.Itoa(int(i))
	return TValue{Value: []byte(s)}
}

func (t TValue) Int8() int8 {
	i, _ := strconv.Atoi(string(t.Value))
	return int8(i)
}

func Int16(i int16) TValue {
	s := strconv.Itoa(int(i))
	return TValue{Value: []byte(s)}
}

func (t TValue) Int16() int16 {
	i, _ := strconv.Atoi(string(t.Value))
	return int16(i)
}

func Int32(i int32) TValue {
	s := strconv.Itoa(int(i))
	return TValue{Value: []byte(s)}
}

func (t TValue) Int32() int32 {
	i, _ := strconv.Atoi(string(t.Value))
	return int32(i)
}

func Int64(i int64) TValue {
	s := strconv.Itoa(int(i))
	return TValue{Value: []byte(s)}
}

func (t TValue) Int64() int64 {
	i, _ := strconv.Atoi(string(t.Value))
	return int64(i)
}

func Float64(i float64) TValue {
	s := fmt.Sprint(i)
	return TValue{Value: []byte(s)}
}

func (t TValue) Float64() float64 {
	i, _ := strconv.ParseFloat(string(t.Value), 64)
	return i
}

func Uint64(i uint64) TValue {
	s := strconv.FormatUint(i, 10)
	return TValue{Value: []byte(s)}
}

func (t TValue) UInt64() uint64 {
	i, _ := strconv.ParseUint(string(t.Value), 10, 64)
	return i
}
