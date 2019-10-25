package parser

import (
	"regexp"
	"strconv"
	"strings"
)

func parseFinalTime(line string) float64 {
	re := regexp.MustCompile("[-+]?([0-9]*\\.[0-9]+|[0-9]+)$")
	res, _ := strconv.ParseFloat(re.FindString(line), 64)
	return res
}

func parseNMVersion(line string) string {
	re := regexp.MustCompile("[^\\s]+$")
	res := re.FindString(line)
	return res
}

func replaceTrim(line string, replacement string) string {
	return strings.TrimSpace(strings.Replace(line, replacement, "", -1))
}

func parseValue(line string, value string) string {
	tokens := strings.Fields(line)
	for _, s := range tokens {
		if strings.Contains(s, value) {
			return replaceTrim(s, value)
		}
	}
	return ""
}

func parseLine(line string, n int) string {
	tokens := strings.Fields(line)
	if len(tokens) >= n {
		return tokens[n]
	}
	return ""
}

// ParseRunDetails parses run details such as start date/time and estimation time etc.
func ParseRunDetails(lines []string) RunDetails {
	version := DefaultString
	runStart := DefaultString
	runEnd := DefaultString
	estimationTime := DefaultFloat64
	covarianceTime := DefaultFloat64
	functionEvaluations := DefaultInt64
	significantDigits := DefaultFloat64
	problemText := DefaultString
	modFile := DefaultString
	estimationMethod := []string{}
	dataSet := DefaultString
	numberOfPatients := DefaultInt64
	numberOfObs := DefaultInt64
	numberOfDataRecords := DefaultInt64
	outputTable := DefaultString

	for i, line := range lines {
		switch {
		case strings.Contains(line, "1NONLINEAR MIXED EFFECTS MODEL PROGRAM (NONMEM) VERSION"):
			version = parseNMVersion(line)
		case strings.Contains(line, "NO. OF FUNCTION EVALUATIONS USED"):
			functionEvaluations, _ = strconv.ParseInt(replaceTrim(line, "NO. OF FUNCTION EVALUATIONS USED:"), 10, 64)
		case strings.Contains(line, "NO. OF SIG. DIGITS IN FINAL EST.:"):
			significantDigits, _ = strconv.ParseFloat(replaceTrim(line, "NO. OF SIG. DIGITS IN FINAL EST.:"), 64)
		case strings.Contains(line, "Elapsed estimation"):
			estimationTime = parseFinalTime(line)
		case strings.Contains(line, "Elapsed covariance time in seconds:"):
			covarianceTime = parseFinalTime(line)
		case strings.Contains(line, "Elapsed postprocess time in seconds:"):
			covarianceTime = parseFinalTime(line)
		case strings.Contains(line, "Started"):
			runStart = replaceTrim(line, "Started")
		case strings.Contains(line, "Finished"):
			runEnd = replaceTrim(line, "Finished")
		case strings.Contains(line, "Stop Time:"):
			if i+1 < len(lines) {
				runEnd = lines[i+1]
			}
		case strings.Contains(line, "$PROB"):
			problemText = replaceTrim(line, "$PROB")
		case strings.Contains(line, "#METH:"):

			estimationMethod = append(estimationMethod, replaceTrim(line, "#METH:"))
		case strings.Contains(line, "$DATA"):
			dataSet = parseLine(line, 1)
		case strings.Contains(line, "TOT. NO. OF INDIVIDUALS:"):
			numberOfPatients, _ = strconv.ParseInt(replaceTrim(line, "TOT. NO. OF INDIVIDUALS:"), 10, 64)
		case strings.Contains(line, "TOT. NO. OF OBS RECS:"):
			numberOfObs, _ = strconv.ParseInt(replaceTrim(line, "TOT. NO. OF OBS RECS:"), 10, 64)
		case strings.Contains(line, "NO. OF DATA RECS IN DATA SET:"):
			numberOfDataRecords, _ = strconv.ParseInt(replaceTrim(line, "NO. OF DATA RECS IN DATA SET:"), 10, 64)
		// This is not reliable because TABLE statements can span multiple lines
		// TODO: support using multi-line feature, when available
		// case strings.Contains(line, "$TABLE NOPRINT ONEHEADER FILE="):
		// 	outputTable = parseValue(line, "FILE=")
		default:
			continue
		}
	}

	if version == "7.4.3" {
		if runStart == "" || runStart == DefaultString {
			runStart = lines[0]
		}
	}

	return RunDetails{
		Version:             version,
		RunStart:            runStart,
		RunEnd:              runEnd,
		EstimationTime:      estimationTime,
		CovarianceTime:      covarianceTime,
		FunctionEvaluations: functionEvaluations,
		SignificantDigits:   significantDigits,
		ProblemText:         problemText,
		ModFile:             modFile,
		EstimationMethod:    estimationMethod,
		DataSet:             dataSet,
		NumberOfPatients:    numberOfPatients,
		NumberOfObs:         numberOfObs,
		NumberOfDataRecords: numberOfDataRecords,
		OutputTable:         outputTable,
	}
}
