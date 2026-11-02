# GPS2MQTT

Get your GPS data from various cheap GPS trackers directly (well, via MQTT) into [Home Assistant](https://www.home-assistant.io/) without the massive load of JAVA or third party hosting.

Inspired by a desire to leave [Traccar](https://traccar.org) which, don't get me wrong is a wonderful application, just overkill for simply triggering on GPS events.

I don't currently have the desire to implement the more advance features of protocols (some support writing back to the device) but this may change in future.

## Supported Protocols

* watch
* h02
* gt06
* huabao (JT/T 808, including the OBD2 extension — RPM, coolant temperature, battery voltage, MAF, throttle position, VIN, ICCID, ...)

## Configuration

Configuration consists of blocks, mqtt, meta, and protocol.

### mqtt block

Provides configuration for connecting to your MQTT server

### status block

Provides configuation for the http status endpoint

### filter block

Holds back implausible GPS fixes, similar to Traccar's position filters. A rejected fix is published with the last accepted coordinates and a `position_filter` attribute naming the reason, so the rest of the data still arrives. Currently applies to the huabao protocol.

* `Enabled` - turn the filter on or off (default `true`)
* `MaxAge` - don't publish reports older than this, or older than a report already published; trackers replay stored reports after a lost connection, often out of order (default `"5m"`, `"0s"` disables the age limit). Needs a correct tracker clock; timestamps more than a day off are not judged.
* `Invalid` - reject fixes the tracker flags as not valid (default `true`)
* `Zero` - reject 0,0 coordinates (default `true`)
* `MinSatellites` - reject fixes with fewer satellites, if the tracker reports them (default `0`, disabled)
* `MaxSpeed` - reject jumps implying a speed above this many km/h (default `250`, `0` disables)
* `StaticSpeed` - hold the position while the reported speed is below this many km/h, which removes drift while parked (default `1`, `0` disables)
* `MaxAcceleration`, `MaxDeceleration`, `JumpDistance` - reject jumps longer than the vehicle could have driven between two fixes, given its speed at both ends and how fast it can accelerate and brake (m/s²), plus `JumpDistance` metres of GPS error tolerance (defaults `4`, i.e. 0-100 km/h in 7 s, `8` and `50`; `MaxAcceleration = 0` disables)
* `DryRun` - don't change any coordinates, only mark what the filter would have done in `position_filter` (default `false`)

The vehicle's own (OBD) speed is preferred over the GPS speed when the tracker reports it. Time between fixes is measured from when the position last changed, so a tracker that repeats a frozen position while driving and then catches up isn't mistaken for a jump.

### meta blocks

Each meta block defines a known tracker ID, trackers that connect and try to communicate will not be permitted to do so unless they have a corresponding meta block
These blocks also let you define a friendly name and an icon for [Home Assistant](https://www.home-assistant.io/) to use

### protocol blocks

Let you define a listening port and some timeouts

## Sample configuration

```toml
[mqtt]
ClientName = "gps2mqtt"
Keepalive = "1m"
PingTimeout = "5s"
Username = "bob"
Password = "auntie"
Brokers = [
    "tcp://homassistant.local:1883"
]

[status]
Enabled = false
Listen = "localhost:8080"

[filter]
Enabled = true
MaxSpeed = 250
StaticSpeed = 1

[meta."SA*91678358119"]
Name = "Motorbike Tracker"
Icon = "mdi:motorbike"

[meta."2214050251"]
Name = "Lawnmower Tracker"
Icon = "mdi:robot-mower"

[protocol.watch]
Listen=":5093"

[protocol.h02]
Listen=":5013"

[protocol.huabao]
Listen=":5015"
```
