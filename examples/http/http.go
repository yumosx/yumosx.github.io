package main

import (
	"fmt"
	"net/http"
)

func main() {
	url := "http://baidu.com"

	resp, err := http.Get(url)
	if err != nil {
		fmt.Println(err)
	}

	defer func() {
		err := resp.Body.Close()
		if err != nil {
			err = fmt.Errorf("failed to close the body %w", err)
			panic(err)
		}
	}()
	line1 := resp.Proto + " " + resp.Status
	fmt.Printf("the first line of response:\n%s\n", line1)
}
