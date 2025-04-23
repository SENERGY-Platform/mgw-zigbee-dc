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

package connector

import (
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/model"
	"github.com/SENERGY-Platform/models/go/models"
	"reflect"
	"testing"
)

func TestGetServiceIds(t *testing.T) {
	dt := model.DeviceType{Services: []model.Service{
		{
			LocalId:    "s1",
			Attributes: []models.Attribute{{Key: MgwZigbeeServiceSelectionConditionAttrKey, Value: "contains(value, 'operation') && value.operation == 'on'"}},
		},
	}}

	t.Run("match", func(t *testing.T) {
		result, err := getServiceIds(dt, []byte(`{"operation":"on"}`))
		if err != nil {
			t.Error(err)
			return
		}
		if !reflect.DeepEqual(result, []string{"get", "s1"}) {
			t.Error(result)
			return
		}
	})

	t.Run("no match", func(t *testing.T) {
		result, err := getServiceIds(dt, []byte(`{"operation":"off"}`))
		if err != nil {
			t.Error(err)
			return
		}
		if !reflect.DeepEqual(result, []string{"get"}) {
			t.Error(result)
			return
		}
	})

	t.Run("missing field checked by contains", func(t *testing.T) {
		result, err := getServiceIds(dt, []byte(`{"foo":"off"}`))
		if err != nil {
			t.Error(err)
			return
		}
		if !reflect.DeepEqual(result, []string{"get"}) {
			t.Error(result)
			return
		}
	})

	t.Run("no map checked by contains", func(t *testing.T) {
		result, err := getServiceIds(dt, []byte(`"foo"`))
		if err != nil {
			t.Error(err)
			return
		}
		if !reflect.DeepEqual(result, []string{"get"}) {
			t.Error(result)
			return
		}
	})

	t.Run("handle error", func(t *testing.T) {
		dt = model.DeviceType{Services: []model.Service{
			{
				LocalId:    "s1",
				Attributes: []models.Attribute{{Key: MgwZigbeeServiceSelectionConditionAttrKey, Value: "value.operation == 'on'"}},
			},
		}}
		result, err := getServiceIds(dt, []byte(`"foo"`))
		if err != nil {
			t.Error(err)
			return
		}
		if !reflect.DeepEqual(result, []string{"get"}) {
			t.Error(result)
			return
		}
	})

	t.Run("multi match", func(t *testing.T) {
		dt := model.DeviceType{Services: []model.Service{
			{
				LocalId:    "s1",
				Attributes: []models.Attribute{{Key: MgwZigbeeServiceSelectionConditionAttrKey, Value: "contains(value, 'operation') && value.operation == 'on'"}},
			},
			{
				LocalId:    "s2",
				Attributes: []models.Attribute{{Key: MgwZigbeeServiceSelectionConditionAttrKey, Value: "contains(value, 'operation') && value.operation =~ 'o.*'"}},
			},
			{
				LocalId:    "s3",
				Attributes: []models.Attribute{{Key: MgwZigbeeServiceSelectionConditionAttrKey, Value: "contains(value, 'operation') && value.operation == 'foo'"}},
			},
			{
				LocalId: "s4",
			},
		}}
		result, err := getServiceIds(dt, []byte(`{"operation":"on"}`))
		if err != nil {
			t.Error(err)
			return
		}
		if !reflect.DeepEqual(result, []string{"get", "s1", "s2"}) {
			t.Error(result)
			return
		}
		result, err = getServiceIds(dt, []byte(`{"operation":"off"}`))
		if err != nil {
			t.Error(err)
			return
		}
		if !reflect.DeepEqual(result, []string{"get", "s2"}) {
			t.Error(result)
			return
		}
	})

	t.Run("multi match err", func(t *testing.T) {
		dt := model.DeviceType{Services: []model.Service{
			{
				LocalId:    "s1",
				Attributes: []models.Attribute{{Key: MgwZigbeeServiceSelectionConditionAttrKey, Value: "value.operation == 'on'"}},
			},
			{
				LocalId:    "s2",
				Attributes: []models.Attribute{{Key: MgwZigbeeServiceSelectionConditionAttrKey, Value: "value.operation =~ 'o.*'"}},
			},
			{
				LocalId:    "s3",
				Attributes: []models.Attribute{{Key: MgwZigbeeServiceSelectionConditionAttrKey, Value: "value.operation > 3"}},
			},
			{
				LocalId: "s4",
			},
		}}
		result, err := getServiceIds(dt, []byte(`{"operation":"on"}`))
		if err != nil {
			t.Error(err)
			return
		}
		if !reflect.DeepEqual(result, []string{"get", "s1", "s2"}) {
			t.Error(result)
			return
		}
		result, err = getServiceIds(dt, []byte(`{"operation":"off"}`))
		if err != nil {
			t.Error(err)
			return
		}
		if !reflect.DeepEqual(result, []string{"get", "s2"}) {
			t.Error(result)
			return
		}

		result, err = getServiceIds(dt, []byte(`{"operation":42}`))
		if err != nil {
			t.Error(err)
			return
		}
		if !reflect.DeepEqual(result, []string{"get", "s3"}) {
			t.Error(result)
			return
		}
	})
}
