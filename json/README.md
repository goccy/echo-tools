# Synopsis

Switch JSON Serializer to faster one ( github.com/goccy/go-json )

```
package main

import (
    "jsontools "github.com/goccy/echo-tools/json"
    "github.com/labstack/echo/v4"
)

func main() {
   	e := echo.New()
	e.JSONSerializer = jsontools.NewSerializer()
}
```