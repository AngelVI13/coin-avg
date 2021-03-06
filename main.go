package main

import (
	"os"
	"log"
	"net/http"
	"fmt"
	"math"
	"strings"
	"io/ioutil"
	"encoding/xml"
	"strconv"
	"github.com/xlsx"
	"github.com/similar-text"
)

var USD_EUR_EXCHANGE_RATE float64
var EXCEL_FILENAME string

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

	country_list := make([]string, 0)
		
	for _, sheet := range xlFile.Sheets {
		for _, row := range sheet.Rows {
			coin_data := row.Cells
			if len(coin_data) > 0 {
				coin_ctry := coin_data[0].String()

				if contains(country_list, coin_ctry) == false {
					country_list = append(country_list, coin_ctry)
				}
			}
		}
	}
	fmt.Println("List of identified countries:")
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
		// __________________End of formating_________________

		countries = append(countries, country_str)
	}
	
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
}

func ExtractCoinValuesForCountry(country string) []string {
	extracted_coin_list := make([]string, 0)
	
	// First find total number of pages
	first_page_url := "https://en.ucoin.net/catalog/?country=" + country + "&page=1"
	body_str := GetPageSouce(first_page_url)
	pages_raw_str := GetStringInBetween(body_str, "<div class=\"pages\">", "</div>")

	if pages_raw_str == "" {
		fmt.Println("Wrong country: ", country)
		return extracted_coin_list
	}
	
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
	coin_num := strings.Count(coin_table, "<table class=\"coin\"")
	coin_data_arr := strings.SplitAfter(coin_table, "</table>")
	
	coin_list := make([]string, 0)
	for i := 0; i<coin_num; i++ {
		// !! cover those cases
		// The GetString... fn can fail if the start&end don't occur,
		// i.e. index -1

		// get title
		ts := GetStringInBetween(coin_data_arr[i], "span class=\"left flag", "</a>")
		title := strings.Split(ts, "</span>")[1]

		// Remove weird char from title string
		if strings.Contains(title, " ") {
			title = strings.Replace(title, " ", " ", -1)
		}

		// Get reference number in format of KM# X where X is a number
		ref_num_str := GetStringInBetween(coin_data_arr[i], "<div class=\"gray-11 km\">", "</div>")
		ref_num_str_index := strings.Index(ref_num_str, "KM#")

		ref_num := ""
		// KM# X number was not found -> Extract UC# X number (this is for commemorative coins)
		if ref_num_str_index == -1 {
			ref_num_str_index = strings.Index(ref_num_str, "UC#")
			// If not referefence number is found then assign NULL string to it
			if ref_num_str_index == -1 {
				ref_num = "NULL"
			}
		}

		// If reference number is found (i.e. not equal to NULL) -> extract it
		if strings.Compare(ref_num, "NULL") != 0 {
			// Extracted reference number, either KM# X or UC# X
			ref_num = ref_num_str[ref_num_str_index:len(ref_num_str)]			
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

			// Iterate over coin list per year and add the reference number and an empty/invalid
			// description (NULL)
			for _, coin := range coin_list_per_year {
				coin_with_ref_and_desc := coin + "|" + ref_num + "|" + "NULL"
				coin_list = append(coin_list, coin_with_ref_and_desc)
			}
		} else {
			// Get coin value
			worth := GetStringInBetween(coin_data_arr[i], "class=\"green-11\">", "</a>") 
			// Remove unwanted chars
			worth = strings.Replace(worth, "$", "", -1)
			worth = strings.Replace(worth, " ", "", -1)
			// Make sure it is not an empty string and set to 0.0 if it is
			if strings.Compare(worth, "") == 0 {
				worth = "0.0"
			} 
			// Convert value to float
			coin_value, err := strconv.ParseFloat(worth, 64)
			// Convert value from USD to EUR
			coin_value_euros := 0.0
			if err == nil {
				coin_value_euros = coin_value * USD_EUR_EXCHANGE_RATE	
			} else {
				log.Fatalln(err)
			} 

			// Get coin description. Only valid for commemorative coins. It holds information about what
			// is commemorated i.e. Princess Diana, XXVII Summer Olympic Games Sydney 2000 etc
			coin_descr_start := "<div class=\"dgray-13\">"
			coin_descr_end := "</div>"
			coin_description := ""
			if strings.Contains(coin_data_arr[i], coin_descr_start) {
				coin_description = GetStringInBetween(coin_data_arr[i], coin_descr_start, coin_descr_end)
			}
			
			//Compose a combined coin information string
			combined_str := title + "|" + strconv.FormatFloat(coin_value_euros, 'f', 2, 64) + "|" + ref_num + "|" + coin_description
			coin_list = append(coin_list, combined_str)
			//fmt.Printf("Coin-Title : %s \n Coin-Worth : %s \n\n", title, worth)
		}
	}

	return coin_list
}

func ExtractCoinValueForIndividualYear(coin_url string, coin_title string) []string {
	body_str := GetPageSouce(coin_url)
	year_table := GetStringInBetween(body_str, "<h3 class=\"th\">Mintage, Worth:</h3>", "</table>")
	year_table = GetStringInBetween(year_table, "<tbody>", "</tbody>")

	year_table_row := strings.SplitAfter(year_table, "</tr>")

	// Remove range of years from title so that it can later be replaced
	// with an individual year - i.e. 5 marka 2015-2017 -> 5 marka 2016
	_, title_stripped := ExtractYearFromTitle(coin_title)
		
	coin_list := make([]string, 0)
	for _, table_row := range year_table_row {
		year_row := GetStringInBetween(table_row, "<td>", "</td>")
		coin_year := GetStringInBetween(year_row, "\">", "<")

		// Extract coin value and covert to Euro
		coin_val_str := GetStringInBetween( table_row, "#price\">", "</a>")

		if strings.Compare(coin_val_str, "") == 0 {
			coin_val_str = "0.0"
		}
		coin_val := strings.Replace(coin_val_str, ",", "", -1) // remove ',' from coin value (1,234.25 -> 1234.25)
		coin_value, err := strconv.ParseFloat(coin_val, 64)

		coin_value_euros := 0.0
		if err == nil {
			coin_value_euros = coin_value * USD_EUR_EXCHANGE_RATE	
		} else {
			log.Fatalln(err)
		} 

		if strings.Compare(coin_year, "") != 0 {
			coin_info := title_stripped + " " + coin_year + "|" + strconv.FormatFloat(coin_value_euros, 'f', 2, 64)
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
			fmt.Println("Exchange rate EUR->USD: ", v.Rate)
			rate_value := strings.Replace(v.Rate, ",", "", -1) // remove ',' from rate (1,234.25 -> 1234.25)
			exc_rate, err := strconv.ParseFloat(rate_value, 64)

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

func ConvertYearToGregorianCalendar(coin_country, coin_title, coin_year string) (c_title, c_year string) {
	// Sometimes the conversion of islamic years is not 100% accurate
	// so it might be needed to send a range of years
	// i.e if the computation results in 1393 -> 1972.78 => send back 1972-1973
	// Convertion formula: CE = ((M x 970224)/1000000)+ 621.5774
	// Since the country string is going to be extracted from the excel document
	// Use a map to find out if a string is part of this map
	islam_countries := map[string]bool {
		"Iran": true,
		"Libya": true,
		"Morocco": true,
		"Sudan": true,
		"Kuwait": true,
		"Egypt": true,
		"Tunisia": true,
		"Afghanistan": true,
		"Yemen": true,
		"Syria": true,
		"Iraq": true,
		"United Arab Emirates": true,
		"Saudi Arabia": true,
		"Jordan": true,
		"Qatar": true,
		"Oman": true,
		"Bahrain": true,
	}

	// Japanese years correspond to the number of years the current emperor has ruled
	// Therefore to find the current year, add the starting year of the rule
	// to the current year (-1 because he ruled in the 0th year)
	// Taisho 14 -> Taisho 1 (1912) + 14 - 1 = 1925
	// Starting years: Taisho (1912), Showa (1926), Heisei (1989)
	japanese_periods := [3]string{
		"Taisho", "Showa", "Heisei",
	}

	// Taiwan starts counting years from the founding of
	// the Republic of China i.e. 1912 is year 1
	// In order to find the current year: 1912 + taiwan year - 1 = current year
	taiwan_start_year := 1912

	converted_year := "0"
	modified_title := coin_title

	if coin_yr_int, err := strconv.Atoi(coin_year); err == nil {
		if coin_yr_int > 1600 {
			// This is likely a valid year (gregorian cal)
			// return without changing anything
			return coin_title, coin_year 
		} else {
			// Convert year
			switch coin_country {
			case "Taiwan":
				converted_year_int := taiwan_start_year + coin_yr_int - 1
				converted_year = strconv.Itoa(converted_year_int) 
			case "Japan":
				// Japanese coin titles are as follows "Nomination - Period"
				// => Extract period and remove any unnecessary whitespaces 
				title_arr := strings.Split(coin_title, "-")
				coin_period := strings.Replace(title_arr[1], " ", "", -1)

				// Iterate over all known periods and find the best matching one
				best_match := 0
				best_match_period := ""
				for _, period := range japanese_periods {
					match_score := similartxt.SimilarText(coin_period, period)

					if match_score > best_match {
						best_match = match_score
						best_match_period = period
					}
				}

				base_year := 0
				switch best_match_period {
				case "Taisho":
					base_year = 1912
				case "Showa":
					base_year = 1926
				case "Heisei":
					base_year = 1989
				default:
					// do nothing
				}
				converted_year_int := base_year + coin_yr_int - 1
				converted_year = strconv.Itoa(converted_year_int)
				// Also return a modified coin_title that does not
				// contain the period. this is needed to ensure better
				// matching with the results of the extracted coins
				modified_title := title_arr[0]
				// Remove any trailing whitespace of title
				if strings.HasSuffix(modified_title, " ") {
					modified_title = modified_title[:len(modified_title)-len(" ")]
				} 
				
			default:
				// if coin country is an islamic country 
				if islam_countries[coin_country] {
					// Convertion formula: CE = ((M x 970224)/1000000)+ 621.5774
					converted_year_float := ( ( float64(coin_yr_int) * 970224.0) / 1000000.0)+ 621.5774
					// if year is 2014.78, returns 0.78
					converted_year_decimals := converted_year_float - math.Floor(converted_year_float)
					converted_year_int := 0
					// Years with high decimals are rounded up in order to ensure
					// better year prediction
					if converted_year_decimals > 0.80 {
						converted_year_int = int(math.Floor(converted_year_float)) + 1
					} else {
						converted_year_int = int(math.Floor(converted_year_float))
					}
					
					converted_year = strconv.Itoa(converted_year_int)
				} else {
					// This is not a country or year that needs to be modified
					// return without changing anything
					return coin_title, coin_year
				}
			}
		} 
	}
	
	return modified_title, converted_year
}

func ExtractYearFromTitle(coin_info string) (string, string) {
	// Coin year from the extracted coin info
	coin_info_arr := strings.SplitAfter(coin_info, " ")
	// Ignore the first 2 parts of this array since they are the nominal and the coin
	// i.e. 5 euro ...
	coin_info_arr_small := coin_info_arr[2:len(coin_info_arr)]

	year_str_index := 0
	year_str := ""
	for index, str_part := range coin_info_arr_small {
		// Remove any spaces,commas or dashes
		str_part_edit := strings.Replace(str_part, ",", "", -1)
		str_part_edit = strings.Replace(str_part_edit, "-", "", -1)
		str_part_edit = strings.Replace(str_part_edit, " ", "", -1)
		_, err := strconv.ParseInt(str_part_edit, 10, 64)
		if err == nil {
			// We have found the year related string part
			year_str = strings.Replace(str_part, ",", "", -1)
			year_str = strings.Replace(year_str, " ", "", -1) 
			year_str_index = index
		}
	}
		
	// Coin title (i.e. Nominal + Name of coin (marka, fening etc)) 
	title_no_year := ""
	for word_c := 0; word_c < len(coin_info_arr); word_c++ {
		// Do not add the year part into the title string
		// We need to add 2 to the year index because in the above
		// For loop we exclude the first 2 parts(nominal and type) of the string slice
		if word_c != year_str_index + 2 {
			title_no_year = title_no_year + coin_info_arr[word_c] 
		}	
	}
	
	return year_str, title_no_year
}

func MatchCoinsAndWriteToExcel(filename, country string, coins []string) {
	excelFileName := filename

	xlFile, err := xlsx.OpenFile(excelFileName)

	if err != nil {
		fmt.Printf(err.Error())
	}
	
	for _, sheet := range xlFile.Sheets {
		for _, row := range sheet.Rows {
			if len(row.Cells) < 5 {
				continue
			}
			coin_data := row.Cells
			coin_ctry := coin_data[0].String() // Col A
			coin_rf := coin_data[1].String() // Col B
			coin_ttle := coin_data[3].String() // Col D
			coin_yr := coin_data[4].String() // Col E
			// If coin country is islamic and year is <1500 -> convert to Gregorian Calendar
			// If coin country is Japan, Taiwan and year is <200 -> convert to Gregorian Calendar 
			coin_ttle, coin_yr = ConvertYearToGregorianCalendar(coin_ctry, coin_ttle, coin_yr)
			coin_i := coin_ttle + " " + coin_yr 

			// Technically both country strings should match perfectly since they
			// were both extracted from the same document
			// The condition below is supposed to guard for any encoding/decoding
			// errors when parsing non ascii chars.
			// consider removing the comparison after experimentation
			country_match_score := similartxt.SimilarText(coin_ctry, country)
			coin_best_match := 0
			coin_value_best_match := ""
			coin_descrp_best_match := ""
			coin_info := ""
			coin_match := ""
			
			if country_match_score >= 90 {
				for _, coin := range coins {
					coin_arr := strings.Split(coin, "|")
					coin_info = coin_arr[0]
					coin_value := coin_arr[1]
					coin_ref_num := coin_arr[2]
					coin_description := coin_arr[3]
					
					// Extracts coin nominal string with the trailing SPACE
					ext_coin_nominal := strings.Split(coin_info, " ")[0] + " " 
					coin_nominal := strings.Split(coin_ttle, " ")[0] + " "

					// Only match coins with the same nominal and the same year
					if strings.Compare(ext_coin_nominal, coin_nominal) == 0 {
						coin_year_ext, ext_coin_title := ExtractYearFromTitle(coin_info)
						// fmt.Println(coin_info)
						// fmt.Println(coin_year_ext)
						// fmt.Println(coin_year, coin_yr)

						if strings.Compare(coin_year_ext, coin_yr) == 0 {
							// fmt.Println(coin_ttle, ext_coin_title)
							// Check if the coin in question is a commemorative coin
							// that is if it has a description inside ()
							if strings.Contains(coin_ttle, "(") /* && len(strings.Split(coin_ttle, " ")) > 1 */ {
								// if the coin that we are trying to match to the excel
								// also has a description (i.e. commemorative information)
								if strings.Compare(coin_description, "NULL") != 0 {
									// Create full title+(description) for extracted coin
									// and compare with excel title (which already has the same format)
									ext_coin_ttle_full := ext_coin_title + "(" + coin_description + ")"
									coin_match_score := similartxt.SimilarText(coin_ttle, ext_coin_ttle_full)
									//fmt.Printf("Trying to match (%d): %s *WITH* %s\n", coin_match_score, coin_ttle, ext_coin_ttle_full) 
									
									if coin_match_score >= coin_best_match {
										coin_value_best_match = coin_value
										coin_best_match = coin_match_score
										coin_match = coin_info
										coin_descrp_best_match = coin_description
									}
								} else {
									// skip coins that don't have the commemorative descrp
									continue
								}	
							} else {
								//fmt.Printf("Trying to compare: %s -> %s \n", coin_rf, coin_ref_num)
								// its not a commemorative coin -> compare reference numbers (if they are not NULL or empty)
								if strings.Contains(coin_rf, "KM#") && !strings.Contains(coin_ref_num, "NULL") {
									if strings.Compare(coin_rf, coin_ref_num) == 0 {
										coin_match_score := similartxt.SimilarText(coin_ttle, ext_coin_title)

										if coin_match_score >= coin_best_match {
											coin_value_best_match = coin_value
											coin_best_match = coin_match_score
											coin_match = coin_info
											coin_descrp_best_match = coin_description
										}
									}
								} else {
									// No reference number is present -> compare just titles
									coin_match_score := similartxt.SimilarText(coin_ttle, ext_coin_title)

									if coin_match_score >= coin_best_match {
										coin_value_best_match = coin_value
										coin_best_match = coin_match_score
										coin_match = coin_info
										coin_descrp_best_match = coin_description
									} 
								} 
							}							
						}
					}
				}
				if coin_best_match == 0 {
					fmt.Printf("M(0): %s -> No match was found\n", coin_i)
					euro_cell := row.AddCell()
					euro_cell.Value = "X"
					match_score_cell := row.AddCell()
					match_score_cell.Value = "X"
					match_coin_cell := row.AddCell()
					match_coin_cell.Value = "No match was found"
				} else {
					euro_cell := row.AddCell()
					euro_cell.Value = coin_value_best_match
					match_score_cell := row.AddCell()
					match_score_cell.Value = strconv.Itoa(coin_best_match)
					match_coin_cell := row.AddCell()
					match_coin_cell.Value = coin_match + " | " + coin_descrp_best_match
					fmt.Printf("M(%d): %s -> %s | Value: %s | Commem: %s\n", coin_best_match, coin_i, coin_match, coin_value_best_match, coin_descrp_best_match)
				}
			}			
		}
	}
	
	err = xlFile.Save(EXCEL_FILENAME)
	if err != nil {
		fmt.Printf(err.Error())
	}
}

func main() {
	// os.Args[0] returns the executable name
	// while os.Args[1] returns the 1st parameter
	if len(os.Args) > 1 {
		EXCEL_FILENAME = os.Args[1]
		if strings.Contains(EXCEL_FILENAME, "xlsx") {		
			country_list := ReadExcelDoc(EXCEL_FILENAME)
			country_params := ConvertCountryListToUrlParameters(country_list)

			USD_EUR_EXCHANGE_RATE = GetExchangeRateUsdToEuro()
			
			// iterate over country list
			for index, country := range country_list {
				// the extractor fn requires the country in the format expected by the website
				coin_list := ExtractCoinValuesForCountry(country_params[index])

				if len(coin_list) == 0 {
					continue
				}
				MatchCoinsAndWriteToExcel(EXCEL_FILENAME, country, coin_list) 
			}

			fmt.Println("Coin value extraction finished successfully!")
		} else {
			fmt.Println("Please provide a path(wrapped in \"\") to an xlsx file with coin information as an argument to this executable") 
		} 
	} else {
		fmt.Println("Please provide a path(wrapped in \"\") to an xlsx file with coin information as an argument to this executable")
	} 
}
