package parser

import (
	"reflect"
	"testing"
)

var finalParameterEstimates01 = []string{
	"********************                             FINAL PARAMETER ESTIMATE                           ********************",
	" ********************                                                                                ********************",
	" ************************************************************************************************************************",
	" ",
	"",
	" THETA - VECTOR OF FIXED EFFECTS PARAMETERS   *********",
	"",
	"         TH 1      TH 2      TH 3      TH 4     ",
	" ",
	"         4.79E+00  9.02E+01  7.47E+00  1.05E+02",
	" ",
	"",
	" OMEGA - COV MATRIX FOR RANDOM EFFECTS - ETAS  ********",
	"",
	"            ETA1      ETA2   ",
	" ",
	" ETA1",
	"+        1.58E-01",
	" ",
	" ETA2",
	"+        1.22E-01  1.33E-01",
	" ",
	"",
	" SIGMA - COV MATRIX FOR RANDOM EFFECTS - EPSILONS  ****",
	"",
	"            EPS1      EPS2   ",
	" ",
	" EPS1",
	"+        1.45E+03",
	" ",
	" EPS2",
	"+        0.00E+00  7.39E-03",
	" ",
	"1",
	"",
	" OMEGA - CORR MATRIX FOR RANDOM EFFECTS - ETAS  *******",
	"",
	"            ETA1      ETA2   ",
	" ",
	" ETA1",
	"+        3.98E-01",
	" ",
	" ETA2",
	"+        8.42E-01  3.65E-01",
	" ",
	"",
	" SIGMA - CORR MATRIX FOR RANDOM EFFECTS - EPSILONS  ***",
	"",
	"            EPS1      EPS2   ",
	" ",
	" EPS1",
	"+        3.81E+01",
	" ",
	" EPS2",
	"+        0.00E+00  8.60E-02",
	" ",
	"1",
}

var finalParameterEstimates01Results = ParametersResult{
	[]float64{4.79, 90.2, 7.47, 105},
	[]float64{0.158, 0.122, 0.133},
	[]float64{1450, 0, 0.00739},
}

func TestParseFinalParameterEstimates(t *testing.T) {
	parsedData := ParseFinalParameterEstimatesFromLst(finalParameterEstimates01)
	if !reflect.DeepEqual(parsedData.Theta, finalParameterEstimates01Results.Theta) {
		t.Log("Got: ", parsedData.Theta, "Expected: ", finalParameterEstimates01Results.Theta)
		t.Fail()
	}
	if !reflect.DeepEqual(parsedData.Omega, finalParameterEstimates01Results.Omega) {
		t.Log("Got: ", parsedData.Omega, "Expected: ", finalParameterEstimates01Results.Omega)
		t.Fail()
	}
	if !reflect.DeepEqual(parsedData.Sigma, finalParameterEstimates01Results.Sigma) {
		t.Log("Got: ", parsedData.Sigma, "Expected: ", finalParameterEstimates01Results.Sigma)
		t.Fail()
	}
}

var sigmaResults01 = []string{
	"	SIGMA - COV MATRIX FOR RANDOM EFFECTS - EPSILONS  ****",
	"",
	"",
	"      EPS1",
	"",
	"EPS1",
	"+        2.45E-03",
	"",
	"1",
	"",
	"",
	"OMEGA - CORR MATRIX FOR RANDOM EFFECTS - ETAS  *******",
	"",
	"",
	"	ETA1      ETA2      ETA3     ",
	"",
	"ETA1",
	"+        3.17E-01",
	"",
	"ETA2",
	"+        0.00E+00  1.90E-01",
	"",
	"ETA3",
	"+        0.00E+00  0.00E+00  1.06E-01",
	"",
	"",
}
var sigmaResults01Parsed = []float64{
	0.00245,
}

func TestParseBlockResultsSigma(t *testing.T) {
	est := ParseFinalParameterEstimatesFromLst(sigmaResults01)
	if est.Sigma[0] != sigmaResults01Parsed[0] {
		t.Log("GOT: ", est.Sigma[0], " EXPECTED: ", sigmaResults01Parsed[0])
		t.Fail()
	}
	if len(est.Sigma) != len(sigmaResults01Parsed) {
		t.Log("GOT: ", len(est.Sigma), " EXPECTED: ", len(sigmaResults01Parsed))
		t.Fail()
	}
}
