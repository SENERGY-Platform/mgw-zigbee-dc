/*
 * Copyright (c) 2023 InfAI (CC SES)
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
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/SENERGY-Platform/models/go/models"
)

func (this *DeviceRepo) CreateDeviceTypeWithDistinctAttributes(dt models.DeviceType, attributeKeys []string) (result models.DeviceType, code int, err error) {
	this.dtMux.Lock()
	defer this.dtMux.Unlock()

	//prevent creation of duplicate device-types
	key := getDtVendorModelKey(dt)
	if existing, ok := this.createdDt[key]; ok {
		return existing, 200, nil
	}

	this.lastDtRefresh = time.Time{} //force new http request on next FindDeviceTypeId() call

	buf := &bytes.Buffer{}
	err = json.NewEncoder(buf).Encode(dt)
	if err != nil {
		return result, 500, err
	}
	req, err := http.NewRequest(http.MethodPost, this.config.DeviceRepositoryUrl+"/device-types?distinct_attributes="+url.QueryEscape(strings.Join(attributeKeys, ",")), buf)
	if err != nil {
		return result, 500, err
	}
	if this.config.AuthEnabled() {
		token, err := this.getToken()
		if err != nil {
			return result, 500, err
		}
		req.Header.Set("Authorization", token)
	}
	result, code, err = Do[models.DeviceType](req)
	if err != nil {
		return result, code, err
	}

	//don't create this device type again
	this.createdDt[key] = result

	//ensure device type has time to be accessible before continuing
	time.Sleep(1 * time.Second)

	return result, code, err
}

func Do[T any](req *http.Request) (result T, code int, err error) {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return result, http.StatusInternalServerError, err
	}
	defer resp.Body.Close()
	if resp.StatusCode > 299 {
		temp, _ := io.ReadAll(resp.Body) //read error response end ensure that resp.Body is read to EOF
		return result, resp.StatusCode, errors.New(string(temp))
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		_, _ = io.ReadAll(resp.Body) //ensure resp.Body is read to EOF
		return result, http.StatusInternalServerError, err
	}
	return result, resp.StatusCode, nil
}

func getDtVendorModelKey(dt models.DeviceType) string {
	attrMap := map[string]string{}
	for _, attr := range dt.Attributes {
		attrMap[attr.Key] = attr.Value
	}
	return fmt.Sprintf("vendor=%v;model=%v", attrMap[AttributeZigbeeVendor], attrMap[AttributeZigbeeModel])
}
