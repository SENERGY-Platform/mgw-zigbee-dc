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
	"context"
	"fmt"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/configuration"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/devicerepo/auth"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/devicerepo/fallback"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/model"
	"sync"
	"time"
)

type DeviceRepo struct {
	config                    configuration.Config
	auth                      *auth.Auth
	fallback                  fallback.Fallback
	deviceTypes               []model.DeviceType
	minCacheDuration          time.Duration
	maxCacheDuration          time.Duration
	lastDtRefresh             time.Time
	lastDtRefreshUsedFallback bool
	dtMux                     sync.Mutex
}

func New(ctx context.Context, config configuration.Config) (*DeviceRepo, error) {
	minCacheDuration, err := time.ParseDuration(config.MinCacheDuration)
	if err != nil {
		return nil, err
	}
	maxCacheDuration, err := time.ParseDuration(config.MaxCacheDuration)
	if err != nil {
		return nil, err
	}
	f, err := fallback.NewFallback(config.FallbackFile)
	if err != nil {
		return nil, err
	}
	return &DeviceRepo{
		config:           config,
		fallback:         f,
		minCacheDuration: minCacheDuration,
		maxCacheDuration: maxCacheDuration,
	}, nil
}

func (this *DeviceRepo) getToken() (string, error) {
	if this.auth == nil {
		this.auth = &auth.Auth{}
	}
	return this.auth.EnsureAccess(this.config)
}

func (this *DeviceRepo) FindDeviceTypeId(device model.ZigbeeDeviceInfo) (string, error) {
	deviceTypes, err := this.ListZigbeeDeviceTypes()
	if err != nil {
		return "", err
	}
	deviceType, ok := this.getMatchingDeviceType(deviceTypes, device)
	if !ok && time.Since(this.lastDtRefresh) > this.minCacheDuration {
		err = this.refreshDeviceTypeList()
		if err != nil {
			return "", err
		}
		deviceTypes, err = this.ListZigbeeDeviceTypes()
		if err != nil {
			return "", err
		}
		deviceType, ok = this.getMatchingDeviceType(deviceTypes, device)
	}
	if !ok {
		return "", fmt.Errorf("%w: vendor=%v model=%v", model.NoMatchingDeviceTypeFound, device.Definition.Vendor, device.Definition.Model)
	}
	return deviceType.Id, nil
}

const AttributeZigbeeVendor = "senergy/zigbee-vendor"
const AttributeZigbeeModel = "senergy/zigbee-model"

func (this *DeviceRepo) getMatchingDeviceType(devicetypes []model.DeviceType, device model.ZigbeeDeviceInfo) (model.DeviceType, bool) {
	for _, dt := range devicetypes {
		attrMap := map[string]string{}
		for _, attr := range dt.Attributes {
			attrMap[attr.Key] = attr.Value
		}
		if vendor, vendorIsSet := attrMap[AttributeZigbeeVendor]; vendorIsSet && vendor == device.Definition.Vendor {
			if m, modelIsSet := attrMap[AttributeZigbeeModel]; modelIsSet && m == device.Definition.Model {
				return dt, true
			}
		}
	}
	return model.DeviceType{}, false
}
