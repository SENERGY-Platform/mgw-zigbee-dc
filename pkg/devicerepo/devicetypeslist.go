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
	"errors"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/model"
	"log"
	"time"
)

const AttributeUsedForZigbee = "senergy/zigbee-dc"
const DtFallbackKey = "device-types"

func (this *DeviceRepo) ListZigbeeDeviceTypes() (list []model.DeviceType, err error) {
	age := time.Since(this.lastDtRefresh)
	if (this.lastDtRefreshUsedFallback && age > this.minCacheDuration) || age > this.maxCacheDuration {
		err = this.refreshDeviceTypeList()
		if err != nil {
			return nil, err
		}
	}
	return this.getDeviceTypeList(), nil
}

func (this *DeviceRepo) refreshDeviceTypeList() error {
	this.dtMux.Lock()
	defer this.dtMux.Unlock()
	result, err := this.getDeviceTypeListFromPermissionSearch()
	if err == nil {
		this.deviceTypes = result
		this.lastDtRefresh = time.Now()
		this.lastDtRefreshUsedFallback = false
		err = this.fallback.Set(DtFallbackKey, this.deviceTypes)
		if err != nil {
			log.Println("WARNING: unable to store device-types in fallback file")
		}
		return nil
	} else {
		log.Println("WARNING: use fallback file to load device type list")
		result, err = this.getDeviceTypeListFromFallback()
		if err != nil {
			return err
		}
		this.deviceTypes = result
		this.lastDtRefresh = time.Now()
		this.lastDtRefreshUsedFallback = true
		return nil
	}
}

func (this *DeviceRepo) getDeviceTypeListFromPermissionSearch() (result []model.DeviceType, err error) {
	token, err := this.getToken()
	if err != nil {
		return result, err
	}
	err, _ = PermissionSearch(token, this.config.PermissionsSearchUrl, QueryMessage{
		Resource: "device-types",
		Find: &QueryFind{
			QueryListCommons: QueryListCommons{
				Limit:  9999,
				Offset: 0,
				Rights: "r",
				SortBy: "name",
			},
			Filter: &Selection{
				Condition: &ConditionConfig{
					Feature:   "features.attributes.key",
					Operation: QueryEqualOperation,
					Value:     AttributeUsedForZigbee,
				},
			},
		},
	}, &result)
	if err != nil {
		return result, err
	}
	return result, nil
}

func (this *DeviceRepo) getDeviceTypeListFromFallback() (result []model.DeviceType, err error) {
	value, fallbackerr := this.fallback.Get(DtFallbackKey)
	if err != nil {
		log.Println("ERROR: unable to load fallback", fallbackerr)
		return result, errors.Join(err, fallbackerr)
	}
	var ok bool
	result, ok = value.([]model.DeviceType)
	if !ok {
		err = errors.New("fallback file does not contain expected device-type format")
		log.Println("ERROR:", err)
		return result, err
	}
	return result, nil
}

func (this *DeviceRepo) getDeviceTypeList() []model.DeviceType {
	this.dtMux.Lock()
	defer this.dtMux.Unlock()
	return this.deviceTypes
}
