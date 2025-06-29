package verapack

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/DanCreative/veracode-go/veracode"
)

type result struct {
	// PassedPolicy indicates whether the scan passed the SAST & SCA policy rules set for the application.
	// It does not include scan frequency- or scan type rules, and it does not take into account grace periods.
	PassedPolicy bool

	// The policy status of the application on the platform.
	// Values can be: "Did Not Pass", "Pass" or "Conditional Pass".
	// Will not be set for sandbox scans.
	PolicyStatus string
}

// WaitForResult blocks the goroutine and periodically polls the API to check whether the scan has completed. Once it has, it returns whether the
// build meets certain policy rules or not.
func WaitForResult(ctx context.Context, client *veracode.Client, options Options, r reporter) (result, string, error) {
	id, guid, err := getApplicationIdentifiers(ctx, client, options.AppName)
	if err != nil {
		return result{}, err.Error(), err
	}
	options.AppId = id
	options.AppGuid = guid

	id, prevPolicyUpdateDate, err := getLatestBuild(ctx, client, options)
	if err != nil {
		return result{}, err.Error(), err
	}

	scheduledTimeoutTime := time.Now().Add(time.Duration(options.ScanTimeout) * time.Minute)

	timeoutChan, waitChan := make(chan struct{}), make(chan struct{})

	go func() {
		for {
			if time.Now().After(scheduledTimeoutTime) {
				break
			}
			time.Sleep(1 * time.Second)
		}

		close(timeoutChan)
	}()

	go func() {
		for {
			select {
			case <-waitChan:
				time.Sleep(time.Second * time.Duration(options.ScanPollingInterval))
				waitChan <- struct{}{}

			case <-timeoutChan:
				return
			}
		}
	}()

	for {
		buildStatus, policyUpdated, err := getBuildStatus(ctx, client, options, id, prevPolicyUpdateDate)
		if err != nil {
			return result{}, err.Error(), err
		}

		switch buildStatus {
		case "Incomplete", "Prescan Failed", "No Modules Defined":
			return result{}, fmt.Sprintf("the scan (buildId=%d) has failed with status: '%s'. Please review the scan on the platform for more information.", id, buildStatus), fmt.Errorf("the scan has failed")

		case "Results Ready":
			if policyUpdated || options.ScanType == ScanTypeSandbox {
				// The policy is only updated after the results are made ready
				// The policy is also only updated on Policy Scans
				SummaryResult, err := getResult(ctx, client, options, id)
				if err != nil {
					return result{}, err.Error(), err
				}

				return SummaryResult, "", nil
			}
		}

		waitChan <- struct{}{}

		select {
		case <-timeoutChan:
			return result{}, fmt.Sprintf("The scan duration exceeded the timeout set: %d min", options.ScanTimeout), errors.New("timeout error")

		case <-waitChan:
		}
	}
}

func getApplicationIdentifiers(ctx context.Context, client *veracode.Client, name string) (id int, guid string, err error) {
	appList, _, err := client.Application.ListApplications(ctx, veracode.ListApplicationOptions{Name: name})
	if err != nil {
		return 0, "", err
	}

	if len(appList) == 0 {
		// This should be impossible because the previous step in the process
		// should catch it. I am placing a check here just to be safe.
		return 0, "", fmt.Errorf("could not find an application with name: '%s'", name)
	}

	for _, app := range appList {
		if strings.EqualFold(app.Profile.Name, name) {
			id = app.Id
			guid = app.Guid
			return
		}
	}

	// Again, this should be impossible at this point, but I am placing a check regardless.
	return 0, "", fmt.Errorf("could not find an application with name: '%s'", name)
}

func getLatestBuild(ctx context.Context, client *veracode.Client, options Options) (id int, policyUpdateDate time.Time, err error) {
	bi, _, err := client.UploadXML.GetBuildInfo(ctx, veracode.BuildInfoOptions{AppId: options.AppId, SandboxId: options.SandboxId})
	if err != nil {
		return 0, time.Time{}, err
	}

	id, _ = strconv.Atoi(bi.BuildId)
	policyUpdateDate = bi.Build.PolicyUpdatedDate

	return
}

// getBuildStatus will return the latest status of the build with provided buildId.
func getBuildStatus(ctx context.Context, client *veracode.Client, options Options, buildId int, prevPolicyUpdateDate time.Time) (status string, policyUpdated bool, err error) {
	bi, _, err := client.UploadXML.GetBuildInfo(ctx, veracode.BuildInfoOptions{AppId: options.AppId, SandboxId: options.SandboxId, BuildId: buildId})
	if err != nil {
		return
	}

	status = bi.Build.AnalysisUnit.Status

	// There seems to be an inconsistent delay between when the XML Upload API reports that the policy scan is completed,
	// and when the summary_report reports it. Adding an arbitrary delay here to remove or at least reduce the likelihood of this issue.
	policyUpdated = bi.Build.PolicyUpdatedDate.After(prevPolicyUpdateDate) && bi.Build.PolicyUpdatedDate.Add(1*time.Minute).Before(time.Now())
	return
}

func getResult(ctx context.Context, client *veracode.Client, options Options, buildId int) (result, error) {
	summaryReportOptions := veracode.SummaryReportOptions{
		BuildId: buildId,
	}

	isSandbox := options.ScanType == ScanTypeSandbox

	if isSandbox {
		summaryReportOptions.Context = options.SandboxGuid
	}

	summaryReport, _, err := client.Application.GetSummaryReport(ctx, options.AppGuid, summaryReportOptions)
	if err != nil {
		return result{}, err
	}

	// Example of policy scan summary for app that did not pass
	// "policy_compliance_status": "Conditional Pass",
	// "policy_rules_status": "Did Not Pass",

	// Example of sandbox scan summary for the same app that did not pass
	// "policy_compliance_status": "Did Not Pass",
	// "policy_rules_status": "Pass",

	// Therefore, I need to use policy_compliance_status for sandbox scans and policy_rules_status for prod scans to determine whether a scan "passed".
	if isSandbox {
		return result{PassedPolicy: summaryReport.PolicyComplianceStatus == "Pass"}, nil
	} else {
		return result{PassedPolicy: summaryReport.PolicyRulesStatus == "Pass", PolicyStatus: summaryReport.PolicyComplianceStatus}, nil
	}
}
