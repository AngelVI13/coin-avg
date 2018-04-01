package main

import (
	"log"
	"net/http"
	"fmt"
	"strings"
	"io/ioutil"
	"encoding/xml"
	"strconv"
	"github.com/xlsx"
	"github.com/similar-text"
)

var USD_EUR_EXCHANGE_RATE float64

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

func GetPageSouce(url_string string) string {
	client := &http.Client{}

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

	return body_str
	//fmt.Println(body_str)
}

func ExtractCoinValuesForCountry(country string) []string {
	extracted_coin_list := make([]string, 0)
	
	// First find total number of pages
	first_page_url := "https://en.ucoin.net/catalog/?country=" + country + "&page=1"

	body_str := GetPageSouce(first_page_url)

	pages_raw_str := GetStringInBetween(body_str, "<div class=\"pages\">", "</div>")

	pagination_arr := strings.SplitAfter(pages_raw_str, "</a>")

	// The splitAfter method will return the last element as empty string
	// because </a> is found at the end of the orignal string.
	// Therefore we take the second to last element from the array
	last_page_elem := pagination_arr[len(pagination_arr)-2]

	last_page_str := GetStringInBetween(last_page_elem, "\">", "</")
	total_num_pages, err := strconv.Atoi(last_page_str)

	if err != nil {
		log.Fatalln(err)
	} else {
		// Loop over all pages and obtain coin info
		for page := 1; page <= total_num_pages; page++ {
			page_url := "https://en.ucoin.net/catalog/?country=" + country + "&page=" + strconv.Itoa(page)
			coin_values := ExtractCoinValuesPerPage(page_url)
			extracted_coin_list = append(extracted_coin_list, coin_values...)
		}
	}
	return extracted_coin_list
}

func ExtractCoinValuesPerPage(url string) []string {
	body_str := GetPageSouce(url)

	start_region := "clear:both;padding-top:5px;\">"
	end_region := "</div><div style=\"float: left; width: 300px;\">" 
	
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
			worth = "0.0"
		}

		// get title
		ts := GetStringInBetween(coin_data_arr[i], "span class=\"left flag", "</a>")
		title := strings.Split(ts, "</span>")[1]

		// Remove weird char from title string
		if strings.Contains(title, " ") {
			title = strings.Replace(title, " ", " ", -1)
		}

		// Check if title has a range of years i.e. "50 fenings 2010-2017"
		if strings.Contains(title, "-") {
			// if title is a range of years then the price for each
			// year has to be extracted. In order to do that
			// we generate the specific coin url and call a fn
			// to extract the information for each year
			coin_url := GetStringInBetween(coin_data_arr[i], "<div class=\"coin-desc\"><a href=\"", "\" title=")
			coin_url = "https://en.ucoin.net/" + coin_url
			coin_list_per_year := ExtractCoinValueForIndividualYear(coin_url, title)
			coin_list = append(coin_list, coin_list_per_year...)
		} else {
			worth = strings.Replace(worth, "$", "", -1)
			worth = strings.Replace(worth, " ", "", -1)

			if strings.Compare(worth, "") == 0 {
				worth = "0.0"
			} 

			coin_value, err := strconv.ParseFloat(worth, 64)

			coin_value_euros := 0.0
			if err == nil {
				coin_value_euros = coin_value * USD_EUR_EXCHANGE_RATE	
			} else {
				log.Fatalln(err)
			} 
			//fmt.Println(coin_value, worth, USD_EUR_EXCHANGE_RATE)
			
			combined_str := title + "|" + strconv.FormatFloat(coin_value_euros, 'f', 2, 64)
			coin_list = append(coin_list, combined_str)
			//fmt.Printf("Coin-Title : %s \n Coin-Worth : %s \n\n", title, worth)
		}
	}

	// fmt.Printf("\nCoin list:\n\n")
	// for _, item := range coin_list {
	// 	fmt.Println(item)
	// }

	return coin_list
}

func ExtractCoinValueForIndividualYear(coin_url string, coin_title string) []string {
	body_str := GetPageSouce(coin_url)
	year_table := GetStringInBetween(body_str, "<h3 class=\"th\">Mintage, Worth:</h3>", "</table>")
	year_table = GetStringInBetween(year_table, "<tbody>", "</tbody>")

	year_table_row := strings.SplitAfter(year_table, "</tr>")

	// Remove range of years from title so that it can later be replaced
	// with an individual year - i.e. 5 marka 2015-2017 -> 5 marka 2016
	title_arr := strings.SplitAfter(coin_title, " ")
	title_stripped := ""
	for word_c := 0; word_c < len(title_arr) - 1; word_c++ {
		title_stripped = title_stripped + title_arr[word_c] 
	}
	//fmt.Println(title_stripped)
	
	coin_list := make([]string, 0)
	for _, table_row := range year_table_row {
		year_row := GetStringInBetween(table_row, "<td>", "</td>")
		coin_year := GetStringInBetween(year_row, "\">", "<")

		// Extract coin value and covert to Euro
		coin_val_str := GetStringInBetween( table_row, "#price\">", "</a>")

		if strings.Compare(coin_val_str, "") == 0 {
			coin_val_str = "0.0"
		}
		coin_value, err := strconv.ParseFloat(coin_val_str, 64)

		coin_value_euros := 0.0
		if err == nil {
			coin_value_euros = coin_value * USD_EUR_EXCHANGE_RATE	
		} else {
			log.Fatalln(err)
		} 

		if strings.Compare(coin_year, "") != 0 {
			coin_info := title_stripped + coin_year + "|" + strconv.FormatFloat(coin_value_euros, 'f', 2, 64)
			coin_list = append(coin_list, coin_info)
		}
	}

	return coin_list
}

// Type used to contain the data obtained from the currency exchange
type Envelope struct {
	Cube []struct {
		Date  string `xml:"time,attr"`
		Rates []struct {
			Currency string `xml:"currency,attr"`
			Rate     string `xml:"rate,attr"`
		} `xml:"Cube"`
	} `xml:"Cube>Cube"`
}

func GetExchangeRateUsdToEuro() float64 {
	// get the latest USD -> EUR exchange rate 
	resp, err := http.Get("http://www.ecb.europa.eu/stats/eurofxref/eurofxref-daily.xml")

	if err != nil {
		log.Fatal(err)
	}

	defer resp.Body.Close()

	xmlCurrenciesData, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		log.Fatal(err)
	}


	var env Envelope
	err = xml.Unmarshal(xmlCurrenciesData, &env)

	if err != nil {
		log.Fatal(err)
	}

	//fmt.Println("Date ", env.Cube[0].Date)
	exchange_rate := 0.0
	for _, v := range env.Cube[0].Rates {
		if strings.Compare(v.Currency, "USD") == 0 {
			fmt.Println("Currency : ", v.Currency, " Rate : ", v.Rate)
			exc_rate, err := strconv.ParseFloat(v.Rate, 64)

			if err != nil {
				log.Fatalln(err)
			} else {
				// The rate obtained above is for EUR -> USD
				// In order to get USD -> EUR we need invert it
				exchange_rate = 1.0 / exc_rate
			} 
		} 
	}

	// REMEMBER! this data is from European Central Bank
	// therefore the rates are based on EUR
	return exchange_rate
}

func MatchCoinsAndWriteToExcel(filename, country string, coins []string) {
	excelFileName := filename

	xlFile, err := xlsx.OpenFile(excelFileName)

	if err != nil {
		fmt.Printf(err.Error())
	}
	
	for _, sheet := range xlFile.Sheets {
		for _, row := range sheet.Rows {
			// fmt.Println(row)
			coin_data := row.Cells
			coin_ctry := coin_data[0].String() // Col A
			coin_ttle := coin_data[3].String() // Col D
			coin_yr := coin_data[4].String() // Col E
			coin_i := coin_ttle + " " + coin_yr 

			// Technically both country strings should match perfectly since they
			// were both extracted from the same document
			// The condition below is supposed to guard for any encoding/decoding
			// errors when parsing non ascii chars.
			// consider removing the comparison after experimentation
			country_match_score := similartxt.SimilarText(coin_ctry, country)
			coin_best_match := 0
			coin_value_best_match := ""
			coin_info := ""
			coin_match := ""
			
			if country_match_score >= 90 {
				for _, coin := range coins {
					coin_arr := strings.Split(coin, "|")
					coin_info = coin_arr[0]
					coin_value := coin_arr[1]

					// Extracts coin nominal string with the trailing SPACE
					ext_coin_nominal := strings.Split(coin_info, " ")[0] + " " 
					coin_nominal := strings.Split(coin_ttle, " ")[0] + " "

					// Only match coins with the same nominal and the same year
					if strings.Compare(ext_coin_nominal, coin_nominal) == 0 {
						// Coin year from the extracted coin info
						coin_info_arr := strings.SplitAfter(coin_info, " ")
						coin_year := coin_info_arr[len(coin_info_arr)-1]
						// Coin title (i.e. Nominal + Name of coin (marka, fening etc))
						ext_coin_title := ""
						for word_c := 0; word_c < len(coin_info_arr) - 1; word_c++ {
							ext_coin_title = ext_coin_title + coin_info_arr[word_c] 
						}
						
						if strings.Compare(coin_year, coin_yr) == 0 {
							coin_match_score := similartxt.SimilarText(coin_ttle, ext_coin_title)

							if coin_match_score > coin_best_match {
								coin_value_best_match = coin_value
								coin_best_match = coin_match_score
								coin_match = coin_info
							}
						}
					}
				}
				if coin_best_match == 0 {
					fmt.Printf("M(0): %s -> No match was found\n", coin_i)
				} else {
					fmt.Printf("M(%d): %s -> %s | Value: %s \n", coin_best_match, coin_match, coin_i, coin_value_best_match)
				}
			}			
		}
	}
}

func main() {
	//ExtractCoinValuesForCountry("bhutan")

	//ReadExcelDoc("/home/angel/go/CoinAvg/coin_sample.xlsx")
	//ReadExcelDoc("/home/angel/go/CoinAvg/tauras651_coins.xlsx")
	//ReadExcelDoc("C:\\Users\\angel.iliev\\go\\coin-avg-master\\tauras651_coins.xlsx")
	//excel_filename := "C:\\Users\\angel.iliev\\go\\coin-avg-master\\coin_sample.xlsx"
	excel_filename := "/home/angel/go/CoinAvg/coin_sample.xlsx"
	country_list := ReadExcelDoc(excel_filename)
	country_params := ConvertCountryListToUrlParameters(country_list)
	//fmt.Println(country_params)

	USD_EUR_EXCHANGE_RATE = GetExchangeRateUsdToEuro()
	
	// iterate over country list
	for index, country := range country_list {
		// the extractor fn requires the country in the format expected by the website
		coin_list := ExtractCoinValuesForCountry(country_params[index])
		//fmt.Println(coin_list, country)
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

	// NOTE: The original excel has more columns than just E,
	//       Be careful when writing to the file so as not to
	//       Overwrite existing data

	// NOTE1: First match nominal, then match year, only after that match coin!!!!!!
	
	// TIP: instead of constantly reading the excel file
	//      create a coin struct that will contain all info
	//      + the index (row) of the coin in question
	//      In this way, you can iterate over the already
	//      loaded array of coins and we don't lose time
	//      constantly reading and writing from/to file
}
