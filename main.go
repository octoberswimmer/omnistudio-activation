package main

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	force "github.com/ForceCLI/force/lib"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

const packageNamespace = "omnistudio__"

func compileOSAndFlexCards() {
	session, err := force.ActiveForce()
	if err != nil {
		panic(err.Error())
	}

	instanceUrl := session.Credentials.InstanceUrl
	accessToken := session.Credentials.AccessToken

	queryOmniscript := `SELECT Id, UniqueName FROM OmniProcess WHERE IsActive = true AND IsIntegrationProcedure = false`
	result, err := session.Query(queryOmniscript)
	if err != nil {
		panic(err.Error())
	}
	var omniScriptIds []string
	for _, record := range result.Records {
		omniScriptIds = append(omniScriptIds, record["Id"].(string))
	}

	queryFlexCard := `SELECT Id, UniqueName FROM OmniUiCard WHERE IsActive = true`
	result, err = session.Query(queryFlexCard)
	if err != nil {
		panic(err.Error())
	}
	var flexCardIds []string
	for _, record := range result.Records {
		flexCardIds = append(flexCardIds, record["Id"].(string))
	}

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.NoSandbox,
		chromedp.DisableGPU,
	)
	if headless := os.Getenv("HEADLESS"); headless != "" {
		h, _ := strconv.ParseBool(headless)
		opts = append(opts, chromedp.Flag("headless", h))
	}
	logger := func(string, ...interface{}) {
	}
	if debug := os.Getenv("DEBUG"); debug != "" {
		d, _ := strconv.ParseBool(debug)
		if d {
			logger = log.Printf
			opts = append(opts, chromedp.IgnoreCertErrors)
		}
	}

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(
		allocCtx,
		chromedp.WithDebugf(logger),
	)
	defer cancel()

	// Create a timer that waits for the network to be idle for idleDuration
	idleDuration := 2 * time.Second
	timer := time.NewTimer(idleDuration)

	// Set up listeners
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		// fmt.Printf("Received event: %T\n", ev)
		switch ev.(type) {

		case *network.EventRequestWillBeSent:
			// Reset the timer as there's ongoing network activity
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(idleDuration)
		}

	})

	ch := make(chan struct{})
	waitNetworkIdle := func() chromedp.Action {
		return chromedp.ActionFunc(func(ctx context.Context) error {
			// Wait for the timer to expire indicating network has been idle for idleDuration
			go func() {
				<-timer.C
				close(ch)
			}()

			// Block until the network has been idle for idleDuration
			<-ch
			return nil
		})
	}

	waitForUrl := func(expected string) chromedp.Action {
		return chromedp.ActionFunc(func(ctx context.Context) error {
			var currentURL string
			for {
				// Get current URL
				if err := chromedp.Location(&currentURL).Do(ctx); err != nil {
					return err
				}
				// Check if it matches the expected URL
				u, err := url.Parse(currentURL)
				if err != nil {
					return fmt.Errorf("Could not parse URL, %s: %w", currentURL, err)
				}

				if strings.Contains(u.Path, expected) {
					return nil
				}
				// If not, sleep for a while and then check again
				log.Println("current URL", currentURL, "does not match expected", expected)
				time.Sleep(100 * time.Millisecond)
			}
		})
	}

	// Timeout the entire browser session after 10 minutes
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	if err := chromedp.Run(timeoutCtx,
		chromedp.Navigate(instanceUrl+"/secur/frontdoor.jsp?sid="+accessToken),
		waitNetworkIdle(),
	); err != nil {
		log.Fatalf("Failed navigating to login page: %v", err)
	}

	log.Printf("Activating OmmiScripts %+v\n", omniScriptIds)
	for _, omniScriptId := range omniScriptIds {
		omniScriptDisignerpageLink := instanceUrl + "/apex/" + packageNamespace + "OmniLwcCompile?id=" + omniScriptId + "&activate=true"
		log.Println("Loading", omniScriptDisignerpageLink)
		var currentStatus string

		timeoutCtx, cancelParse := context.WithTimeout(ctx, 5*time.Minute)
		loadTimeCtx, cancelLoad := context.WithTimeout(timeoutCtx, 30*time.Second)
	SCRIPT:
		for {
			if err := chromedp.Run(loadTimeCtx,
				chromedp.Navigate(omniScriptDisignerpageLink),
				waitForUrl("OmniLwcCompile"),
			); err != nil {
				log.Fatalf("Failed loading OmniScript compilation page: %v", err)
			}
		STATUS:
			for {
				if err := chromedp.Run(timeoutCtx,
					chromedp.WaitVisible("#compiler-message"),
					chromedp.Text("#compiler-message", &currentStatus),
				); err != nil {
					log.Fatalf("Failed checking OmniScript compilation status: %v", err)
				}
				switch {
				case currentStatus == "DONE":
					log.Println("LWC Activated successfully")
					break SCRIPT
				case strings.HasPrefix(currentStatus, "ERROR: No MODULE named markup"):
					log.Println("Missing Custom LWC - " + currentStatus)
				case strings.HasPrefix(currentStatus, "ERROR"):
					log.Println("Error Activating LWC - " + currentStatus)
					break STATUS
				default:
					log.Println("Status: " + currentStatus)
				}
				time.Sleep(2 * time.Second)
			}
		}
		cancelLoad()
		cancelParse()
	}

	if len(flexCardIds) > 0 {
		log.Printf("Activating FlexCards %+v\n", flexCardIds)
		flexCardCompilePage := instanceUrl + "/apex/" + packageNamespace + "FlexCardCompilePage?id=" + strings.Join(flexCardIds, ",")
		log.Println("Loading", flexCardCompilePage)
		var currentStatus, jsonError string

		timeoutCtx, cancelParse := context.WithTimeout(ctx, 5*time.Minute)
		loadTimeCtx, cancelLoad := context.WithTimeout(timeoutCtx, 30*time.Second)
		defer cancel()
		if err := chromedp.Run(loadTimeCtx,
			chromedp.Navigate(flexCardCompilePage),
			waitForUrl("FlexCardCompilePage"),
		); err != nil {
			log.Fatalf("Failed loading Flex Card compilation page: %v", err)
		}
	CARD:
		for {
			if err := chromedp.Run(timeoutCtx,
				chromedp.WaitVisible("#compileMessage-0"),
				chromedp.Text("#compileMessage-0", &currentStatus),
				chromedp.WaitVisible("#resultJSON-0"),
				chromedp.Text("#resultJSON-0", &jsonError),
			); err != nil {
				log.Fatalf("Failed checking Flex Card compilation status: %v", err)
			}
			switch {
			case currentStatus == "DONE SUCCESSFULLY":
				log.Println("LWC Activated successfully")
				break CARD
			case currentStatus == "DONE WITH ERRORS":
				log.Println("LWC FlexCards Compilation Error Result:" + jsonError)
			default:
				log.Println("Status: " + currentStatus)
			}
			time.Sleep(2 * time.Second)
		}
		cancelLoad()
		cancelParse()
	}
}

func main() {
	compileOSAndFlexCards()
}
