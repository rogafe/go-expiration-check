package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/openrdap/rdap"

	"github.com/urfave/cli/v2" // imports as package "cli"
)

type RdapServices struct {
	Services [][][]string `json:"services"`
}

type DomainInfo struct {
	DomainName   string  `json:"domain_name"`
	Registrar    string  `json:"registrar"`
	ExpiryDate   string  `json:"expiry_date"`
	DaysToExpire float64 `json:"days_to_expire"`
	Failed       bool    `json:"failed"`
}

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.SetPrefix("go-expiration-check: ")
	log.SetOutput(os.Stderr)
}

func main() {

	app := &cli.App{
		Name:   "go-expiration-check",
		Usage:  "Check for domain expiration",
		Action: cli.ShowAppHelp,

		Commands: []*cli.Command{
			{
				Name:    "check",
				Aliases: []string{"c"},
				Usage:   "Check domain expiration",
				Flags: []cli.Flag{
					&cli.StringSliceFlag{
						Name:    "domain",
						Aliases: []string{"d"},
						Usage:   "Domain names to check, can be a comma-separated list or read from env variable",
					},
					&cli.StringFlag{
						Name:    "env",
						Aliases: []string{"e"},
						Usage:   "Environment variable containing domain names",
					},
					&cli.StringFlag{
						Name:    "output",
						Aliases: []string{"o"},
						Usage:   "Output format: json or text",
						Value:   "text",
					},
				},
				Action: func(cCtx *cli.Context) error {
					var domains []string

					// Get domains from flag
					if cCtx.IsSet("domain") {
						domains = cCtx.StringSlice("domain")
					}

					// Get domains from environment variable
					if cCtx.IsSet("env") {
						envDomains := os.Getenv(cCtx.String("env"))
						if envDomains != "" {
							domains = append(domains, strings.Split(envDomains, ",")...)
						}
					}

					// Get domains from standard input if no domains provided
					if len(domains) == 0 {
						fmt.Println("Enter domain names (comma-separated):")
						reader := bufio.NewReader(os.Stdin)
						input, err := reader.ReadString('\n')
						if err != nil {
							return fmt.Errorf("failed to read from standard input: %v", err)
						}
						input = strings.TrimSpace(input)
						if input != "" {
							domains = append(domains, strings.Split(input, ",")...)
						}
					}

					if len(domains) == 0 {
						return fmt.Errorf("no domain names provided")
					}

					outputFormat := cCtx.String("output")

					var results []DomainInfo

					for _, domain := range domains {
						// fmt.Printf("Checking expiration for domain: %s\n", domain)
						domainInfo, err := GetDomainInfo(domain)
						if err != nil {
							log.Printf("Error checking domain %s: %v", domain, err)
							continue
						}
						results = append(results, domainInfo)
					}

					if outputFormat == "json" {
						jsonData, err := json.MarshalIndent(results, "", "  ")
						if err != nil {
							return fmt.Errorf("failed to marshal JSON: %v", err)
						}
						fmt.Println(string(jsonData))
					} else {
						for _, result := range results {
							fmt.Printf("Domain: %s\n", result.DomainName)
							fmt.Printf("Registrar: %s\n", result.Registrar)
							fmt.Printf("Expiry Date: %s\n", result.ExpiryDate)
							fmt.Printf("Days to Expire: %.2f\n", result.DaysToExpire)
							fmt.Println()
						}
					}

					return nil
				},
			},
		},
	}
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}

}

func GetDomainInfo(domainName string) (DomainInfo, error) {
	tld := ExtractTLD(domainName)
	rdapServices := GetRdapServices()
	rdapService, ok := rdapServices[tld]
	_ = rdapService
	if ok {
		// log.Printf("Using RDAP service %s for %s\n", rdapService, domainName)
		return GetRdap(domainName)
	} else {
		// return GetWhois(domainName)
		return DomainInfo{}, fmt.Errorf("No RDAP service found for %s", domainName)
	}

}

// func GetWhois(domain string) (DomainInfo, error) {
// 	domainFailed := DomainInfo{
// 		DomainName: domain,
// 		Failed:     true,
// 	}
// 	whois_raw, err := whois.Whois(domain)
// 	if err != nil {
// 		return domainFailed, err
// 	}

// 	log.Println(whois_raw)

// 	result, err := whoisparser.Parse(whois_raw)
// 	if err != nil {
// 		return domainFailed, err
// 	}

// 	log.Println(result.Domain.ExpirationDate)
// 	expiryDate, err := time.Parse("2006-01-02", result.Domain.ExpirationDate)

// 	domainSuccess := DomainInfo{
// 		DomainName: domain,
// 		Registrar:  result.Registrar.Name,
// 		ExpiryDate: expiryDate
// 	}

// 	return domainFailed, nil
// }

func GetRdap(domainName string) (DomainInfo, error) {
	client := &rdap.Client{}
	domain, err := client.QueryDomain(domainName)
	if err != nil {
		return DomainInfo{}, err
	}

	var registrar string
	if len(domain.Entities) > 0 {
		for _, entity := range domain.Entities {
			if entity.Roles != nil && contains(entity.Roles, "registrar") {
				registrar = entity.VCard.Name()
				break
			}
		}
	}

	var expiryDate time.Time
	for _, event := range domain.Events {
		if event.Action == "expiration" {

			expiryDate, err = time.Parse(time.RFC3339, event.Date)
			if err != nil {
				return DomainInfo{}, err
			}
			break
		}
	}

	daysToExpire := float64(time.Until(expiryDate).Hours() / 24)
	domainInfo := DomainInfo{
		DomainName:   domainName,
		Registrar:    registrar,
		ExpiryDate:   expiryDate.Format(time.RFC3339),
		DaysToExpire: daysToExpire,
	}

	return domainInfo, nil
}

func ExtractTLD(domain string) string {
	// Split the domain name by the dot separator
	parts := strings.Split(domain, ".")
	// Get the last part of the domain name
	return parts[len(parts)-1]
}

func GetRdapServices() map[string]string {
	response, _ := http.Get("https://data.iana.org/rdap/dns.json")
	values := make(map[string]string)

	defer response.Body.Close()
	var data RdapServices
	json.NewDecoder(response.Body).Decode(&data)

	for _, value := range data.Services {
		for _, tld := range value[0] {
			values[tld] = value[1][0]
		}
	}

	return values
}

// Helper function to check if a slice contains a string
func contains(slice []string, str string) bool {
	for _, v := range slice {
		if v == str {
			return true
		}
	}
	return false
}
