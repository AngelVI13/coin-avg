package main

import (
	"log"
	"net/http"
	"fmt"
	"strings"
	"io/ioutil"
	"github.com/xlsx"
	"github.com/similar-text"
)

func contains(s []string, e string) bool {
	for _, a := range s {
		if strings.Compare(a, e) == 0 {
			return true
		}
	}
	return false
}

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

func ReadExcelDoc(filename string) []string {
	// excel parsing api https://github.com/tealeg/xlsx
	excelFileName := filename

	xlFile, err := xlsx.OpenFile(excelFileName)

	if err != nil {
		fmt.Printf(err.Error())
	}

	// coin_country := ""
	country_list := make([]string, 0)
	// coin_title := ""
	// coin_date := ""
	
	for _, sheet := range xlFile.Sheets {
		for _, row := range sheet.Rows {
			// fmt.Println(row)
			coin_data := row.Cells
			coin_ctry := coin_data[0].String()

			if contains(country_list, coin_ctry) == false {
				country_list = append(country_list, coin_ctry)
			}
			//coin_country = coin_data[0].String() // Col A
			//coin_title = coin_data[3].String() // Col D
			//coin_date = coin_data[4].String() // Col E
			//fmt.Printf("%s %s %s\n", coin_country, coin_title, coin_date) 
		}
	}
	fmt.Println(country_list)

	return country_list 
}

func ConvertCountryListToUrlParameters(country_list []string) []string {
	// Slice to store the converted country names to expected coin catalog parameters
	countries := make([]string, 0)
	for _, country := range country_list {
		// Split and format country names like Bosnia and Herzegovina to
		// bosnia_herzegovina which is the expected format of the website

		// __________________String formating_________________
		country = strings.ToLower(country)

		if strings.Contains(country, " and ") {
			country = strings.Replace(country, " and ", "_", -1) 
		}
		
		country_conv := strings.Split(country, " ")
		
		country_str := ""
		if len(country_conv) > 1 {
			country_str = country_conv[0] + "_" + country_conv[1]
		} else {
			country_str = country_conv[0]
		}
		// // __________________End of formating_________________

		//fmt.Println(country_str)
		countries = append(countries, country_str)
	}
	//fmt.Println(countries)
	return countries
}

func ExtractCoinValuesForCountry(country string) []string {
	client := &http.Client{}
	
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

	coin_table  := GetStringInBetween(body_str, start_region, end_region)
	//fmt.Println( coin_table )
	coin_num := strings.Count(coin_table, "<table class=\"coin\"")
	//fmt.Println(coin_num)

	coin_data_arr := strings.SplitAfter(coin_table, "</table>")
	//fmt.Println(coin_data_arr[0])
	//fmt.Println(len(coin_data_arr))

	coin_list := make([]string, 0)
	for i := 0; i<coin_num; i++ {
		// !! cover those cases
		// The GetString... fn can fail if the start&end don't occur,
		// i.e. index -1

		// get value
		worth := GetStringInBetween(coin_data_arr[i], "class=\"green-11\">", "</a>")
		if worth == "" {
			worth = "NULL"
		}

		// get title
		ts := GetStringInBetween(coin_data_arr[i], "span class=\"left flag", "</a>")
		title := strings.Split(ts, "</span>")[1]

		// Remove weird char from title string
		if strings.Contains(title, " ") {
			title = strings.Replace(title, " ", " ", -1)
		}
		combined_str := title + "|" + worth
		coin_list = append(coin_list, combined_str)
		fmt.Printf("Coin-Title : %s \n Coin-Worth : %s \n\n", title, worth)
	}
	return coin_list
}

func MatchCoinsAndWriteToExcel(filename, country string, coins []string) {
	excelFileName := filename

	xlFile, err := xlsx.OpenFile(excelFileName)

	if err != nil {
		fmt.Printf(err.Error())
	}

	// coin_country := ""
	// coin_title := ""
	// coin_date := ""
	
	for _, sheet := range xlFile.Sheets {
		for _, row := range sheet.Rows {
			// fmt.Println(row)
			coin_data := row.Cells
			coin_ctry := coin_data[0].String()

			// Technically both strings should match perfectly since they
			// were both extracted from the same document
			// The condition below is supposed to guard for any encoding/decoding
			// errors when parsing non ascii char.
			// consider removing the comparison after experimentation
			match_score := similartxt.SimilarText(coin_ctry, country)
			if match_score >= 70 {
				fmt.Printf("Matched country (%d): %s -> %s\n\n", match_score, country, coin_ctry)
			}
			
			//coin_country = coin_data[0].String() // Col A
			//coin_title = coin_data[3].String() // Col D
			//coin_date = coin_data[4].String() // Col E
			//fmt.Printf("%s %s %s\n", coin_country, coin_title, coin_date) 
		}
	}
}

func main() {
	//ExtractCoinValuesForCountry("bhutan")

	//ReadExcelDoc("/home/angel/go/CoinAvg/coin_sample.xlsx")
	//ReadExcelDoc("/home/angel/go/CoinAvg/tauras651_coins.xlsx")
	//ReadExcelDoc("C:\\Users\\angel.iliev\\go\\coin-avg-master\\tauras651_coins.xlsx")
	excel_filename := "C:\\Users\\angel.iliev\\go\\coin-avg-master\\coin_sample.xlsx"
	country_list := ReadExcelDoc(excel_filename)
	country_params := ConvertCountryListToUrlParameters(country_list)
	fmt.Println(country_params)

	// iterate over country list
	for index, country := range country_list {
		// the extractor fn requires the country in the format expected by the website
		coin_list := ExtractCoinValuesForCountry(country_params[index])
		MatchCoinsAndWriteToExcel(excel_filename, country, coin_list) 
	}
	
	//test := similartxt.SimilarText("Hello World!", "Hello, my beautiful world!")
	//fmt.Println(test)
	// TODO: properly decode strings - now showing weird chars
	// TODO: implement iteration over all pages for a given country 

	// -STEP 1: generate a list of all countries from excel
	// -STEP 2: convert the country string to expected format
	// -STEP 3: iterate over country list and obtain coin info
	// STEP 4: for every coin match an entry from excel & write value
	//   STEP 4.1: if the best match is low (<50%) add a comment
	//             with the original title it was matched to
	//             In this way you can examine manually if the
	//             coin value was matched to the correct coin entry in excel
	
	// TIP: instead of constantly reading the excel file
	//      create a coin struct that will contain all info
	//      + the index (row) of the coin in question
	//      In this way, you can iterate over the already
	//      loaded array of coins and we don't lose time
	//      constantly reading and writing from/to file
}
