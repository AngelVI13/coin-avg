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
	str = str[s:len(str)-1] // remove start from string
	return strings.Split(str, end)[0] // split string at first occurance of end, and return 0th element
}

func main() {
	client := &http.Client{}
	country := "bhutan"
	url_string := "https://en.ucoin.net/catalog/?country=" + country + "&page=1"

	start_region := "clear:both;padding-top:5px;\">"
	end_region := "</div><div style=\"float: left; width: 300px;\">"
	
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

	//trimmed_body := strings.SplitAfter(body_str, "href=\"#price\"")[1]

	coin_table  := GetStringInBetween(body_str, start_region, end_region)
	fmt.Println( coin_table )
	coin_num := strings.Count(coin_table, "<table class=\"coin\"")
	fmt.Println(coin_num)

	coin_data_arr := strings.SplitAfter(coin_table, "</table>")
	fmt.Println(coin_data_arr[0])
	fmt.Println(len(coin_data_arr))

	//fmt.Println(GetStringInBetween(coin_data_arr[0], "class=\"green-11\">", "</a>"))
	
	for i := 0; i<coin_num; i++ {
		// !! cover those cases
		// if value is not present, string might be empty
		// also the GetString... fn can fail if the start&end don't occur,
		// i.e. index -1

		// get value
		worth := GetStringInBetween(coin_data_arr[i], "class=\"green-11\">", "</a>")
		if worth == "" {
			worth = "NULL"
		}

		// get title
		ts := GetStringInBetween(coin_data_arr[i], "span class=\"left flag", "</a>")
		title := strings.Split(ts, "</span>")[1]

		fmt.Printf("Coin-Title : %s \n Coin-Worth : %s \n\n", title, worth)
		//fmt.Println(worth)
		//fmt.Println(title)
	}

	// TODO: properly decode strings - now showing weird chars
	// TODO: implement iteration over all pages for a given country 
}
