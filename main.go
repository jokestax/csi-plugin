package main

import (
	"flag"
	"fmt"
	"github.com/jokestax/csi-plugin/driver"
	"os"
)

func main() {
	var endpoint, token, region string
	flag.Parse()
	endpoint = os.Getenv("CSI_ENDPOINT")
	token = os.Getenv("API_TOKEN")
	fmt.Println("endpoint:", endpoint)
	fmt.Println("token:", token)
	fmt.Println("region:", region)

	drv := driver.NewDriver(region, endpoint)
	if err := drv.Run(); err != nil {
		panic(err)
	}

}
