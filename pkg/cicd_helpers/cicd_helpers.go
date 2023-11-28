package cicd_helpers

import (
    "fmt"
    "os"
)

func SetEnvVariablesForPipeline (multiAddr string) {
	os.Setenv("MASA_NODE_MULTIADDRESS", multiAddr)
    fmt.Println("MASA_NODE_MULTIADDRESS:", os.Getenv("MASA_NODE_MULTIADDRESS"))
}

