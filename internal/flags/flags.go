package flags

import (
	"fmt"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/labels"
)

type File string

func NewFile(str string) *File {
	tmp := File(str)
	return &tmp
}

func (f *File) Get() any {
	return string(*f)
}

func (f *File) Set(str string) error {
	str = strings.TrimSpace(str)

	stat, err := os.Stat(str)
	if err != nil {
		return err
	}

	if !stat.Mode().IsRegular() {
		return fmt.Errorf("(%q) isn't a regular file", str)
	}

	*f = File(str)
	return nil
}

func (f *File) String() string {
	return string(*f)
}

type FileSlice []File

func NewFileSlice(files ...File) *FileSlice {
	tmp := FileSlice(files)
	return &tmp
}

func (fs *FileSlice) Get() any {
	tmp := make([]string, len(*fs))
	for i, f := range *fs {
		tmp[i] = string(f)
	}
	return tmp
}

func (fs *FileSlice) Set(str string) error {
	var tmp FileSlice
	for _, s := range strings.Split(str, ",") {
		var f File
		if err := f.Set(s); err != nil {
			return err
		}

		tmp = append(tmp, f)
	}

	*fs = tmp

	return nil
}

func (fs *FileSlice) String() string {
	tmp := make([]string, len(*fs))
	for i, f := range *fs {
		tmp[i] = string(f)
	}
	return strings.Join(tmp, ",")
}

type LabelSelector string

func NewLabelSelector(str string) *LabelSelector {
	tmp := LabelSelector(str)
	return &tmp
}

func (ls *LabelSelector) Get() any {
	return string(*ls)
}

func (ls *LabelSelector) Set(str string) error {
	_, err := labels.Parse(str)
	if err != nil {
		return err
	}

	*ls = LabelSelector(str)
	return nil
}

func (ls *LabelSelector) String() string {
	return string(*ls)
}

type LogLevel logrus.Level

func NewLogLevel(lvl logrus.Level) *LogLevel {
	tmp := LogLevel(lvl)
	return &tmp
}

func (lvl *LogLevel) Get() any {
	return logrus.Level(*lvl)
}

func (lvl *LogLevel) Set(str string) error {
	tmp, err := logrus.ParseLevel(str)
	if err != nil {
		return err
	}

	*lvl = LogLevel(tmp)
	return nil
}

func (lvl *LogLevel) String() string {
	tmp := logrus.Level(*lvl)
	return tmp.String()
}
