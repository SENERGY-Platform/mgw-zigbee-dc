<a href="https://github.com/SENERGY-Platform/mgw-zigbee-dc/actions/workflows/tests.yml" rel="nofollow">
    <img src="https://github.com/SENERGY-Platform/mgw-zigbee-dc/actions/workflows/tests.yml/badge.svg?branch=main" alt="Tests" />
</a>

TODO

## Auth
The 'auth_endpoint' config field can/should be left empty if this component is used in a MGW with API-Proxy

### Services

#### Set

- local-service-id: "set"
- inputs: json object containing all values that are to be changed
- no outputs

#### Get

- local-service-id: "get"
- no inputs
- outputs: json object with all known device values
  - values/fields may be missing if device has never sent them, and they are not requestable (ref https://www.zigbee2mqtt.io/guide/usage/exposes.html#access)

### Device-Type Attributes

- senergy/zigbee-dc
  - existence indicates that the device-type may be used by this device-connector 
- senergy/zigbee-vendor
  - used to match device-type
  - found in mqtt messages in zigbee2mqtt/bridge/devices 
  - e.g. Philips in $[1].definition.vendor in pkg/tests/resources/device_info_example.json
- senergy/zigbee-model
  - used to match device-type
  - found in mqtt messages in zigbee2mqtt/bridge/devices
  - e.g. 9290012573A in $[1].definition.model in pkg/tests/resources/device_info_example.json

A device whose device-type can not be read from the device-repository (e.g. the mgw mirror is restarting) is not registered and its events are dropped. Such devices are handled again after 1s, the delay doubling up to 1m, until the device-repository answers or a new device list from zigbee2mqtt arrives; once registered, their current values are requested from zigbee2mqtt. A device without matching device-type is not retried.

### Command Acknowledgement

- `COMMAND_ACK_TIMEOUT` (json `command_ack_timeout`, default `30s`)
  - Go duration, e.g. `5s`, `1m`; `-` and `0s` disable the acknowledgement (an empty environment variable is ignored by the env loader and keeps the config.json value; `""` works only in config.json). An invalid value stops the connector at start.
  - A `set` command is answered when zigbee2mqtt publishes a (non-retained) state of the device that contains the values of the set payload; the response `data` is that state.
  - If no such state arrives within the timeout, an error is sent to `error/command/<command_id>` and no response.
  - Compared are the keys of the set payload without `transition` and without keys that the device definition does not expose as published state. Strings are compared case-insensitive, numbers as numbers, `"toggle"` matches any value, object and array values only need their key present.
  - If a state of the device has been received since the connector started, only keys that occur in the last one are compared: some definitions expose a property as published that the device never reports (Aqara SRTS-A01 `external_temperature_input`). This relies on zigbee2mqtt publishing its full cached state (`advanced.cache_state`, the default); a property a device has not reported yet, e.g. right after pairing, is then not confirmed. An empty or unreadable last state counts as unknown.
  - Known limits: a value that zigbee2mqtt normalizes (string presets like `color_temp: "warm"`, values clamped or rounded by the device) never matches and ends in the timeout error although the device reacted. A set to the value the device already has can be confirmed by any later state message, e.g. the echo of a `get`.
  - If the payload is not a JSON object or has no comparable key, or the acknowledgement is disabled, the response is sent immediately with empty `data`.

### Device Attributes

Set on every device this connector reports to the device-management-service, but only if the matching config field or environment variable is set (Go duration, e.g. `1h`, `24h`; not `7d`). An invalid duration stops the connector at start.

- last_message_max_age
  - env: `LAST_MESSAGE_MAX_AGE`
  - read by connection-check-v2: a device whose last message is older counts as offline

The device-cloud-connector writes it to the cloud device with its own attribute origin, so the value stored on the device takes precedence over the account's default attributes in the device-repository.
Offline notifications additionally need the device attribute `monitor_connection_state`, which this connector does not set; it can be set per account as a default attribute in the device-repository.
