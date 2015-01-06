package main

import (
	"testing"
)

var ProductTests = []struct {
	Query, ProdName, URL string
}{
	{
		"MOD021KM.A2009055.1720.005.2009056015521.hdf",
		"MOD03",
		"ftp://ladsweb.nascom.nasa.gov:ftp/allData/5/MOD03/2009/055/MOD03.A2009055.1720.005.2010235111709.hdf",
	},
}

func TestProduct(t *testing.T) {
	for _, pt := range ProductTests {
		p, ok := Products[pt.ProdName]
		if !ok {
			t.Fatalf("bad product name %v\n", pt.ProdName)
		}
		newURL, err := p.GetURL(pt.Query)
		if err != nil {
			t.Fatalf("failed to get URL: %v\n", err)
		}
		if newURL != pt.URL {
			t.Errorf("new URL is %s; expected %s\n", newURL, pt.URL)
		}
	}
}
