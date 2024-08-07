package main

import (
	"encoding/csv"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"slices"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/PuerkitoBio/goquery"
	"golang.org/x/text/runes"
	"golang.org/x/text/secure/precis"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

type ProsVsCons struct {
	prosText string
	consText string
}

type dataCollection struct {
	country      string
	riskMark     string
	businessMark string
	population   string
	rip          string
	growth       string
	inflation    string
	compareTable ProsVsCons
}

func main() {
	myApp := app.New()
	myWindow := myApp.NewWindow("Coface Scraper Tool")
	// Fetch options from the URL
	options, err := fetchOptions("https://www.coface.fr/actualites-economie-conseils/tableau-de-bord-des-risques-economiques", "option")
	slices.Sort(options)
	log.Println("options:", options)
	if err != nil {
		log.Fatalf("Failed to fetch options: %v", err)
	}
	selectedItems := map[string]bool{}

	// Create a label to display selected items
	selectedLabel := widget.NewLabel("Selected: None")

	// Function to update the selected items label
	updateSelectedLabel := func() {
		selected := ""
		for item := range selectedItems {
			if selected != "" {
				selected += ", "
			}
			selected += item
		}
		if selected == "" {
			selected = "None"
		}
		selectedLabel.SetText("Selected: " + selected)
	}

	// Create the search entry
	searchEntry := widget.NewEntry()
	searchEntry.SetPlaceHolder("Search...")

	// Container for dropdown content
	dropdownContent := container.NewVBox()

	// PopUp to show the dropdown
	var popUp *widget.PopUp

	// Function to update the dropdown content
	updateDropdownContent := func(searchText string) {
		// Clear existing content
		dropdownContent.Objects = nil

		for _, option := range options {
			if strings.Contains(strings.ToLower(option), strings.ToLower(searchText)) {
				option := option // capture range variable
				checkbox := widget.NewCheck(option, func(checked bool) {
					if checked {
						selectedItems[option] = true
					} else {
						delete(selectedItems, option)
					}
					updateSelectedLabel()
				})

				// Initialize checkbox state based on selectedItems
				checkbox.SetChecked(selectedItems[option])
				dropdownContent.Add(checkbox)
			}
		}
		dropdownContent.Refresh() // Refresh the container to update the UI
	}

	// Button to show the dropdown menu
	dropdownButton := widget.NewButton("Select Options", func() {
		if popUp != nil {
			popUp.Hide() // Hide existing pop-up if shown
		}

		// Create initial content for the pop-up
		updateDropdownContent("")

		// Wrap the dropdown content in a scroll container
		scrollContainer := container.NewScroll(dropdownContent)
		scrollContainer.SetMinSize(fyne.NewSize(300, 200)) // Set a minimum size for the scroll container

		// Create the pop-up content container
		popUpContent := container.NewBorder(searchEntry, nil, nil, nil, scrollContainer)
		popUp = widget.NewPopUp(popUpContent, myWindow.Canvas())
		popUp.Show()

		// Update the dropdown content based on search entry
		searchEntry.OnChanged = func(searchText string) {
			updateDropdownContent(searchText)
		}
	})

	// Function to reset the selection
	resetSelection := func() {
		selectedItems = map[string]bool{}
		updateSelectedLabel()
	}
	outputFileNameEntry := widget.NewEntry()
	outputFileNameEntry.SetPlaceHolder("Enter output file")

	searchButton := widget.NewButton("Search", func() {
		trans := transform.Chain(
			norm.NFD,
			precis.UsernameCaseMapped.NewTransformer(),
			runes.Map(func(r rune) rune {
				switch r {
				case 'ą':
					return 'a'
				case 'é':
					return 'e'
				case 'è':
					return 'e'
				case 'í':
					return 'i'
				case 'ú':
					return 'u'
				case 'á':
					return 'a'
				case 'ï':
					return 'i'
				case 'î':
					return 'i'
				case 'ć':
					return 'c'
				case 'ę':
					return 'e'
				case 'ł':
					return 'l'
				case 'ń':
					return 'n'
				case 'ó':
					return 'o'
				case 'ś':
					return 's'
				case 'ż':
					return 'z'
				case 'ź':
					return 'z'
				}
				return r
			}),
			norm.NFC,
		)

		if outputFileNameEntry.Text == "" {
			fyne.CurrentApp().SendNotification(&fyne.Notification{
				Title:   "Search Triggered ",
				Content: "No output filename specified",
			})
			return
		}
		//outputFileName := outputFileNameEntry.Text
		var countriesData []dataCollection
		for country, _ := range selectedItems {
			data := dataCollection{
				country:      country,
				riskMark:     "",
				businessMark: "",
				population:   "",
				rip:          "",
				growth:       "",
				inflation:    "",
			}
			noOpenHyphenCountry := strings.Replace(country, "(", "", -1)
			log.Println("COUNTRY: ", noOpenHyphenCountry)
			noCloseHyphenCountry := strings.Replace(noOpenHyphenCountry, ")", "", -1)
			log.Println("COUNTRY: ", noCloseHyphenCountry)
			formatCountry := strings.Replace(strings.ToLower(noCloseHyphenCountry), " ", "-", -1)
			log.Println("COUNTRY: ", formatCountry)
			result, _, _ := transform.String(trans, formatCountry)

			log.Println("COUNTRY: ", result)
			growth, inflation, err := getEconomicIndicators("https://www.coface.fr/actualites-economie-conseils/tableau-de-bord-des-risques-economiques/fiches-risques-pays/" + result)
			data.growth = strings.TrimSpace(growth)
			data.inflation = strings.TrimSpace(inflation)

			if err != nil {
				log.Printf("Error retrieving indicators : %v\n", err)
				continue

			}
			rip, population, _ := getResumeCardLeft("https://www.coface.fr/actualites-economie-conseils/tableau-de-bord-des-risques-economiques/fiches-risques-pays/" + result)
			data.rip = rip
			data.population = population

			risk, business, _ := getResumeCardRight("https://www.coface.fr/actualites-economie-conseils/tableau-de-bord-des-risques-economiques/fiches-risques-pays/" + result)
			data.riskMark = risk
			data.businessMark = business

			pros, cons, _ := getResumeProsCons("https://www.coface.fr/actualites-economie-conseils/tableau-de-bord-des-risques-economiques/fiches-risques-pays/" + result)
			data.compareTable.prosText = pros
			data.compareTable.consText = cons

			log.Printf("%+v", data)
			countriesData = append(countriesData, data)
		}
		writeToCsv(outputFileNameEntry.Text, countriesData)
		return
	})

	resetButton := widget.NewButton("Reset Selection", resetSelection)

	// Create a layout
	content := container.NewVBox(
		widget.NewLabel("Select options:"),
		dropdownButton,
		selectedLabel,
		resetButton,
		widget.NewLabel("Output file name:"),
		outputFileNameEntry,
		searchButton,
	)

	myWindow.SetContent(content)
	myWindow.Resize(fyne.NewSize(600, 500))
	myWindow.ShowAndRun()
}

func fetchOptions(url string, pattern string) ([]string, error) {
	res, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return nil, fmt.Errorf("status code error: %d %s", res.StatusCode, res.Status)
	}

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return nil, err
	}

	var options []string
	doc.Find(pattern).Each(func(i int, s *goquery.Selection) {
		optionText := strings.TrimSpace(s.Text())
		if optionText != "" {
			options = append(options, optionText)
		}
	})

	return options, nil
}

func getEconomicIndicators(url string) (string, string, error) {
	// Fetch the webpage
	res, err := http.Get(url)
	if err != nil {
		log.Printf("Failed to fetch webpage: %v", err)
		return "", "", err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		log.Printf("Status code error: %d %s", res.StatusCode, res.Status)
		return "", "", errors.New(fmt.Sprintf("%s: %d %s", "Status code error", res.StatusCode, res.Status))
	}

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		log.Printf("Failed to parse webpage: %v", err)
		return "", "", err
	}
	var growth string
	var inflation string
	// Find the table with the specified caption
	doc.Find("caption.sr-only:contains('Principaux indicateurs économiques')").Each(func(i int, s *goquery.Selection) {
		// Traverse up to the table element
		table := s.Parent()
		// Iterate over each row in the table
		table.Find("tr").Each(func(j int, row *goquery.Selection) {
			// Find all columns in the row
			if j == 1 {
				cols := row.Find("td")
				if cols.Length() >= 4 { // Ensure there are at least 4 columns
					// Get the text of the 4th column
					growth = cols.Eq(3).Text()
					//fmt.Printf("Row %d, growth: %s\n", j, growth)
				}
			} else if j == 2 {
				cols := row.Find("td")
				if cols.Length() >= 4 { // Ensure there are at least 4 columns
					// Get the text of the 4th column
					inflation = cols.Eq(3).Text()
					//fmt.Printf("Row %d, inflation: %s\n", j, inflation)
				}
			}
		})
	})
	return growth, inflation, nil

}

func getResumeCardLeft(url string) (string, string, error) {
	// Fetch the webpage
	res, err := http.Get(url)
	if err != nil {
		return "", "", err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return "", "", errors.New(fmt.Sprintf("%s: %d %s", "Status code error", res.StatusCode, res.Status))
	}

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		log.Printf("Failed to parse webpage: %v", err)
		return "", "", err
	}
	var rip string
	var population string
	// Find the table with the specified caption
	doc.Find("div.countrySheetHeader__content__card__left").Each(func(i int, s *goquery.Selection) {
		s.Find("dt").Each(func(j int, dt *goquery.Selection) {

			if strings.HasPrefix(dt.Text(), "PIB") {
				dd := dt.NextFiltered("dd")
				if dd != nil {
					rip = strings.TrimSpace(strings.Split(dd.Text(), "$")[0])

				}
			} else if strings.HasPrefix(dt.Text(), "Population") {
				dd := dt.NextFiltered("dd")
				if dd != nil {
					population = strings.TrimSpace(strings.Split(dd.Text(), "Millions")[0])

				}
			}
		})
	})

	return rip, population, nil

}

func getResumeCardRight(url string) (string, string, error) {
	// Fetch the webpage
	res, err := http.Get(url)
	if err != nil {
		log.Printf("Failed to fetch webpage: %v", err)
		return "", "", err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return "", "", errors.New(fmt.Sprintf("%s: %d %s", "Status code error", res.StatusCode, res.Status))
	}

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		log.Printf("Failed to parse webpage: %v", err)
		return "", "", err
	}
	var risk string
	var business string
	// Find the table with the specified caption
	doc.Find("dl.rating").Each(func(i int, s *goquery.Selection) {
		doc.Find("dd").Each(func(i int, s *goquery.Selection) {
			class, exists := s.Attr("class")
			if exists && strings.HasPrefix(class, "color-") {
				if risk == "" {
					risk = strings.Split(class, "color-")[1]
				} else if business == "" {
					business = strings.Split(class, "color-")[1]
				}
			}
		})
	})
	log.Printf("%s %s", risk, business)
	return risk, business, nil

}

func getResumeProsCons(url string) (string, string, error) {
	// Fetch the webpage
	res, err := http.Get(url)
	if err != nil {
		log.Printf("Failed to fetch webpage: %v", err)
		return "", "", err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return "", "", errors.New(fmt.Sprintf("%s: %d %s", "Status code error", res.StatusCode, res.Status))

	}

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		log.Printf("Failed to parse webpage: %v", err)
		return "", "", err
	}
	var prosList strings.Builder
	var consList strings.Builder
	doc.Find("article.prosAndCons__pros li").Each(func(i int, s *goquery.Selection) {
		prosList.WriteString("\n - " + s.Text())
	})
	doc.Find("article.prosAndCons__cons li").Each(func(i int, s *goquery.Selection) {
		consList.WriteString("\n - " + s.Text())
	})

	return prosList.String(), consList.String(), nil

}

func writeToCsv(filename string, data []dataCollection) {
	// Create the CSV file
	file, err := os.Create(strings.Split(filename, ".")[0] + ".csv")
	if err != nil {
		fmt.Println("Failed to create file:", err)
		return
	}
	defer file.Close()

	// Create a CSV writer
	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write the CSV header
	header := []string{
		"Pays",
		"Note risque pays",
		"Note environnement des affaires",
		"Population (en millions)",
		"PIB par habitant (en $US)",
		"Croissance PIB",
		"Inflation (en %)",
	}
	if err := writer.Write(header); err != nil {
		fmt.Println("Failed to write header:", err)
		return
	}

	// Write the CSV data
	for _, d := range data {
		row := []string{
			d.country,
			d.riskMark,
			d.businessMark,
			d.population,
			d.rip,
			d.growth,
			d.inflation,
		}
		if err := writer.Write(row); err != nil {
			fmt.Println("Failed to write row:", err)
			return
		}
	}

	file, err = os.Create(strings.Split(filename, ".")[0] + "_pros_cons.csv")
	if err != nil {
		fmt.Println("Failed to create file:", err)
		return
	}
	defer file.Close()

	// Create a CSV writer
	writer = csv.NewWriter(file)
	defer writer.Flush()

	// Write the CSV header
	header = []string{
		"Pays",
		"Points forts",
		"Points faibles",
	}
	if err := writer.Write(header); err != nil {
		fmt.Println("Failed to write header:", err)
		return
	}

	// Write the CSV data
	for _, d := range data {
		row := []string{
			d.country,
			d.compareTable.prosText,
			d.compareTable.consText,
		}
		if err := writer.Write(row); err != nil {
			fmt.Println("Failed to write row:", err)
			return
		}
	}

	fmt.Println("CSV file created successfully")
}
