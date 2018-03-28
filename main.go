package main

import (
	"log"
	"net/http"
	"fmt"
	"strings"
	"io/ioutil"
)

// GetStringInBetween Returns empty string if no start string found
func GetStringInBetween(str string, start string, end string) (result string) {
	s := strings.Index(str, start)
	if s == -1 {
		return
	}
	s += len(start)
	e := strings.Index(str, end)
	return str[s:e]
}

func main() {
	client := &http.Client{}
	url_string := "https://en.ucoin.net/coin/norway-10-ore-1951-1957/?tid=36641"
	//url_string := "https://en.ucoin.net/coin/lithuania-50-cents-2015-2018/?tid=40563"
	//url_string := "https://en.ucoin.net/coin/lithuania-2-euro-2015/?cid=40565#price"
	
	req, err := http.NewRequest("GET", url_string, nil)
	if err != nil {
		log.Fatalln(err)
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Fatalln(err)
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}

	body_str := string(body)
	//fmt.Println(body_str)
	//fmt.Println(strings.Index(body_str, "href=\"#price\""))

	trimmed_body := strings.SplitAfter(body_str, "href=\"#price\"")[1]

	fmt.Println(GetStringInBetween(trimmed_body, "<span>", "</span>"))
}
