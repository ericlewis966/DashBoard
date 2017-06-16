// Copyright 2017 The Kubernetes Dashboard Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package handler

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/golang/glog"
	"golang.org/x/text/language"
)

const defaultDir = "./public/en"

// Localization is a spec for the localization configuration of dashboard.
type Localization struct {
	Translations []Translation `json:"translations"`
}

// Translation is a single translation definition spec.
type Translation struct {
	File string `json:"file"`
	Key  string `json:"key"`
}

// LocaleHandler serves different localized versions of the frontend application
// based on the Accept-Language header.
type LocaleHandler struct {
	SupportedLocales []string
}

// CreateLocaleHandler loads the localization configuration and constructs a LocaleHandler.
func CreateLocaleHandler() *LocaleHandler {
	locales, err := getSupportedLocales("./locale_conf.json")
	if err != nil {
		glog.Warningf("Error when loading the localization configuration. Dashboard will not be localized. %s", err)
		locales = []string{}
	}
	return &LocaleHandler{SupportedLocales: locales}
}

func getSupportedLocales(configFile string) ([]string, error) {
	// read config file
	localesFile, err := ioutil.ReadFile(configFile)
	if err != nil {
		return []string{}, err
	}

	// unmarshall
	localization := Localization{}
	err = json.Unmarshal(localesFile, &localization)
	if err != nil {
		glog.Warningf("%s %s", string(localesFile), err)
	}

	// filter locale keys
	result := []string{}
	for _, translation := range localization.Translations {
		result = append(result, translation.Key)
	}
	return result, nil
}

// LocaleHandler serves different html versions based on the Accept-Language header.
func (handler *LocaleHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.EscapedPath() == "/" || r.URL.EscapedPath() == "/index.html" {
		// Do not store the html page in the cache. If the user is to click on 'switch language',
		// we want a different index.html (for the right locale) to be served when the page refreshes.
		w.Header().Add("Cache-Control", "no-store")
	}
	acceptLanguage := r.Header.Get("Accept-Language")
	dirName := handler.determineLocalizedDir(acceptLanguage)
	http.FileServer(http.Dir(dirName)).ServeHTTP(w, r)
}

func (handler *LocaleHandler) determineLocalizedDir(locale string) string {
	tags, _, err := language.ParseAcceptLanguage(locale)
	if (err != nil) || (len(tags) == 0) {
		return defaultDir
	}

	for _, tag := range tags {
		matchedLocale := ""
		for _, l := range handler.SupportedLocales {
			base, _ := tag.Base()
			if l == base.String() {
				matchedLocale = l
				break
			}
		}
		localeDir := "./public/" + matchedLocale
		if matchedLocale != "" && handler.dirExists(localeDir) {
			return localeDir
		}
	}
	return defaultDir
}

func (handler *LocaleHandler) dirExists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			glog.Warningf(name)
			return false
		}
	}
	return true
}
