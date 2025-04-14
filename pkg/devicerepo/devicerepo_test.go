/*
 * Copyright 2025 InfAI (CC SES)
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
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/model"
	"github.com/SENERGY-Platform/models/go/models"
	"strings"
	"testing"
)

func TestGetMatchingDeviceType(t *testing.T) {
	deviceTypes := []model.DeviceType{
		{
			Name: "1",
			Attributes: []models.Attribute{
				{
					Key:   AttributeZigbeeVendor,
					Value: "1",
				},
				{
					Key:   AttributeZigbeeModel,
					Value: "1",
				},
			},
		},
		{
			Name: "2",
			Attributes: []models.Attribute{
				{
					Key:   AttributeZigbeeVendor,
					Value: "2",
				},
				{
					Key:   AttributeZigbeeVendor,
					Value: "2.0",
				},
				{
					Key:   AttributeZigbeeModel,
					Value: "2",
				},
			},
		},
		{
			Name: "3",
			Attributes: []models.Attribute{
				{
					Key:   AttributeZigbeeVendor,
					Value: "3",
				},
				{
					Key:   AttributeZigbeeModel,
					Value: "3",
				},
				{
					Key:   AttributeZigbeeModel,
					Value: "3.0",
				},
			},
		},
		{
			Name: "4",
			Attributes: []models.Attribute{
				{
					Key:   AttributeZigbeeVendor,
					Value: "4",
				},
				{
					Key:   AttributeZigbeeVendor,
					Value: "4.0",
				},
				{
					Key:   AttributeZigbeeModel,
					Value: "4",
				},
				{
					Key:   AttributeZigbeeModel,
					Value: "4.0",
				},
			},
		},
	}

	deviceInfos := []model.ZigbeeDeviceInfo{
		{
			Definition: model.ZigbeeDeviceInfoDefinition{
				Model:  "1",
				Vendor: "1",
			},
		},
		{
			Definition: model.ZigbeeDeviceInfoDefinition{
				Model:  "2",
				Vendor: "2",
			},
		},
		{
			Definition: model.ZigbeeDeviceInfoDefinition{
				Model:  "2",
				Vendor: "2.0",
			},
		},
		{
			Definition: model.ZigbeeDeviceInfoDefinition{
				Model:  "3",
				Vendor: "3",
			},
		},
		{
			Definition: model.ZigbeeDeviceInfoDefinition{
				Model:  "3.0",
				Vendor: "3",
			},
		},
		{
			Definition: model.ZigbeeDeviceInfoDefinition{
				Model:  "4",
				Vendor: "4",
			},
		},
		{
			Definition: model.ZigbeeDeviceInfoDefinition{
				Model:  "4.0",
				Vendor: "4",
			},
		},
		{
			Definition: model.ZigbeeDeviceInfoDefinition{
				Model:  "4",
				Vendor: "4.0",
			},
		},
		{
			Definition: model.ZigbeeDeviceInfoDefinition{
				Model:  "4.0",
				Vendor: "4.0",
			},
		},
	}

	for _, deviceInfo := range deviceInfos {
		dt, found := getMatchingDeviceType(deviceTypes, deviceInfo)
		if !found {
			t.Errorf("not found %#v", deviceInfo)
			return
		}
		if !strings.Contains(deviceInfo.Definition.Vendor, dt.Name) {
			t.Errorf("unexpected device-type \n%#v\n%#v\n", dt, deviceInfo.Definition)
			return
		}
	}
	dt, found := getMatchingDeviceType(deviceTypes, model.ZigbeeDeviceInfo{
		Definition: model.ZigbeeDeviceInfoDefinition{
			Model:  "1",
			Vendor: "2",
		},
	})
	if found {
		t.Errorf("unexpected finding of %v for %#v\n%#v\n", dt.Name, model.ZigbeeDeviceInfoDefinition{
			Model:  "1",
			Vendor: "2",
		}, dt)
	}
}
