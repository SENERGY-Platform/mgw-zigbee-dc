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

package fallback

import (
	"log"
	"os"
	"reflect"
	"strconv"
	"testing"
	"time"
)

func TestFallback(t *testing.T) {
	filename := "/tmp/test" + strconv.FormatInt(time.Now().Unix(), 10) + ".json"
	t.Log("use filename=", filename)
	defer func() {
		t.Log("remove", filename)
		err := os.Remove(filename)
		if err != nil {
			log.Println(err)
			t.Error(err)
		}
	}()

	var fallback Fallback
	var fallback2 Fallback

	t.Run("first init", testCreateFallback(&fallback, filename))

	t.Run("set value foo 1", testSetFallbackValue(fallback, "foo", float64(1)))
	t.Run("set value bar batz", testSetFallbackValue(fallback, "bar", "batz"))

	t.Run("check value foo 1", testCheckFallbackValue(fallback, "foo", float64(1)))
	t.Run("check value bar batz", testCheckFallbackValue(fallback, "bar", "batz"))

	t.Run("set value foo 2", testSetFallbackValue(fallback, "foo", float64(2)))
	t.Run("check value foo 2", testCheckFallbackValue(fallback, "foo", float64(2)))

	t.Run("second init", testCreateFallback(&fallback2, filename))

	t.Run("second check value bar batz", testCheckFallbackValue(fallback2, "bar", "batz"))
	t.Run("second check value foo 2", testCheckFallbackValue(fallback2, "foo", float64(2)))
}

func testCheckFallbackValue(fallback Fallback, key string, expected interface{}) func(t *testing.T) {
	return func(t *testing.T) {
		actual, err := fallback.Get(key)
		if err != nil {
			t.Error(err)
			return
		}
		if !reflect.DeepEqual(actual, expected) {
			t.Error(actual, expected)
			return
		}
	}
}

func testSetFallbackValue(fallback Fallback, key string, value interface{}) func(t *testing.T) {
	return func(t *testing.T) {
		err := fallback.Set(key, value)
		if err != nil {
			t.Error(err)
			return
		}
	}
}

func testCreateFallback(f *Fallback, filename string) func(t *testing.T) {
	return func(t *testing.T) {
		temp, err := NewFallback(filename)
		if err != nil {
			t.Error(err)
			return
		}
		*f = temp
	}
}
