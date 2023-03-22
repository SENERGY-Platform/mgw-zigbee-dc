/*
 * Copyright 2022 InfAI (CC SES)
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package devicerepo

import (
	"bytes"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"runtime/debug"
)

func PermissionSearch(token string, searchUrl string, query QueryMessage, result interface{}) (err error, code int) {
	requestBody := new(bytes.Buffer)
	err = json.NewEncoder(requestBody).Encode(query)
	if err != nil {
		return err, http.StatusInternalServerError
	}
	req, err := http.NewRequest("POST", searchUrl+"/v3/query", requestBody)
	if err != nil {
		debug.PrintStack()
		return err, http.StatusInternalServerError
	}
	req.Header.Set("Authorization", token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		debug.PrintStack()
		return err, http.StatusInternalServerError
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		buf := new(bytes.Buffer)
		buf.ReadFrom(resp.Body)
		err = errors.New(buf.String())
		log.Println("ERROR: ", resp.StatusCode, err)
		debug.PrintStack()
		return err, resp.StatusCode
	}
	err = json.NewDecoder(resp.Body).Decode(result)
	if err != nil {
		debug.PrintStack()
		return err, http.StatusInternalServerError
	}

	return nil, http.StatusOK
}

type QueryMessage struct {
	Resource      string         `json:"resource"`
	Find          *QueryFind     `json:"find,omitempty"`
	ListIds       *QueryListIds  `json:"list_ids,omitempty"`
	CheckIds      *QueryCheckIds `json:"check_ids,omitempty"`
	TermAggregate *string        `json:"term_aggregate,omitempty"`
}

type QueryFind struct {
	QueryListCommons
	Search string     `json:"search,omitempty"`
	Filter *Selection `json:"filter,omitempty"`
}

type QueryListIds struct {
	QueryListCommons
	Ids []string `json:"ids,omitempty"`
}

type QueryCheckIds struct {
	Ids    []string `json:"ids,omitempty"`
	Rights string   `json:"rights,omitempty"`
}

type QueryListCommons struct {
	Limit    int    `json:"limit,omitempty"`
	Offset   int    `json:"offset,omitempty"`
	Rights   string `json:"rights,omitempty"`
	SortBy   string `json:"sort_by,omitempty"`
	SortDesc bool   `json:"sort_desc,omitempty"`
}

type QueryOperationType string

const (
	QueryEqualOperation             QueryOperationType = "=="
	QueryUnequalOperation           QueryOperationType = "!="
	QueryAnyValueInFeatureOperation QueryOperationType = "any_value_in_feature"
)

type ConditionConfig struct {
	Feature   string             `json:"feature,omitempty"`
	Operation QueryOperationType `json:"operation,omitempty"`
	Value     interface{}        `json:"value,omitempty"`
	Ref       string             `json:"ref,omitempty"`
}

type Selection struct {
	And       []Selection      `json:"and,omitempty"`
	Or        []Selection      `json:"or,omitempty"`
	Not       *Selection       `json:"not,omitempty"`
	Condition *ConditionConfig `json:"condition,omitempty"`
}

type Permissions struct {
	A bool
	R bool
	W bool
	X bool
}
