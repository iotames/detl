module github.com/iotames/detl

go 1.24.1

require (
	github.com/go-sql-driver/mysql v1.10.0
	github.com/iotames/easyconf v1.2.2
	github.com/iotames/easydb v0.6.0
	github.com/iotames/miniutils v1.0.11
	github.com/lib/pq v1.12.3
	gopkg.in/yaml.v3 v3.0.1
)

require (
	filippo.io/edwards25519 v1.2.0 // indirect
	golang.org/x/net v0.5.0 // indirect
	golang.org/x/text v0.6.0 // indirect
)

replace github.com/iotames/easydb => ./easydb
