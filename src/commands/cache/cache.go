package cache

import (
	"fmt"
	"nvm/bootstrap"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

var Store cacheDir = load()
var root string

type cacheDir struct {
	Metadata string `json:"metadata" label:"Metadata"`
	Versions string `json:"versions" label:"Versions"`
}

func load() cacheDir {
	var c cacheDir

	root, err := bootstrap.CacheRoot()
	if err != nil {
		exe, _ := os.Executable()
		root = filepath.Join(filepath.Dir(exe), ".cache")
	}
	c.Metadata = filepath.Join(root, "metadata")
	c.Versions = filepath.Join(root, "versions")

	return c
}

func (c cacheDir) Get(name string) (label string, path string) {
	t := reflect.TypeOf(c)
	v := reflect.ValueOf(c)
	for i := range t.NumField() {
		f := t.Field(i)
		tag := strings.SplitN(f.Tag.Get("json"), ",", 2)[0]
		if strings.EqualFold(tag, name) {
			return f.Name, v.Field(i).String()
		}
	}
	return "", ""
}

func (c cacheDir) List() map[string][]string {
	result := make(map[string][]string)
	t := reflect.TypeOf(c)
	v := reflect.ValueOf(c)
	for i := range t.NumField() {
		f := t.Field(i)
		label := strings.SplitN(f.Tag.Get("label"), ",", 2)[0]
		attr := strings.SplitN(f.Tag.Get("json"), ",", 2)[0]
		result[strings.ToLower(attr)] = []string{label, v.Field(i).String()}
	}
	return result
}

func (c cacheDir) GetFiles(name string) ([]string, error) {
	var path string
	switch strings.ToLower(name) {
	case "metadata":
		path = c.Metadata
	case "versions":
		path = c.Versions
	default:
		return nil, fmt.Errorf("unknown cache name: %s", name)
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		files = append(files, e.Name())
	}

	return files, nil
}
