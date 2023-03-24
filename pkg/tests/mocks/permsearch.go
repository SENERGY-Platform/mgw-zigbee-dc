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

package mocks

import (
	"context"
	"encoding/json"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/model"
	"net/http"
	"net/http/httptest"
	"sync"
)

type PermissionsSearch struct {
	mux      sync.Mutex
	resp     []model.DeviceType
	respErr  error
	respCode int
}

func (this *PermissionsSearch) SetResp(resp []model.DeviceType, respErr error, respCode int) {
	this.mux.Lock()
	defer this.mux.Unlock()
	this.resp = resp
	this.respErr = respErr
	this.respCode = respCode
}

func StartPermissionsSearch(ctx context.Context, wg *sync.WaitGroup) (url string, ctrl *PermissionsSearch, err error) {
	ctrl = &PermissionsSearch{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		ctrl.mux.Lock()
		defer ctrl.mux.Unlock()
		if ctrl.respErr != nil {
			http.Error(writer, ctrl.respErr.Error(), ctrl.respCode)
			return
		} else {
			json.NewEncoder(writer).Encode(ctrl.resp)
		}
	}))
	wg.Add(1)
	go func() {
		<-ctx.Done()
		server.Close()
		wg.Done()
	}()
	return server.URL, ctrl, nil
}
