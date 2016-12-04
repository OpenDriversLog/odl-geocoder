package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Compufreak345/dbg"
	"github.com/OpenDriversLog/odl-geocoder/models"
	"io/ioutil"
	"net/http"
	"sort"
	"strconv"
	"time"
	"net/url"
	"os"
	"strings"
	"regexp"
)

const TAG = "og/geocode.go"

var ErrEmptyResult = errors.New("Geocoder returned 0 results")
var ErrNoRequestsLeft = errors.New("All geocoders contingent is used up!")

var ErrNeedFixBeforeRetry = errors.New("Need fix before retrying!")
var ErrSkipProvider = errors.New("Skip this provider")
var ErrProviderNotSupported = errors.New("Provider not supported!")
var MaxRequestsPerDay int
var MaxRequestsPerUser int
var CurDailyRequestsUsed int
var CurRequestsByUserUsed map[string]int
var Debug bool
var client = &http.Client{
	Timeout: time.Duration(5 * time.Second),
}
var ChainProviders []*models.GeoCodeProvider
var NonChainProviders []*models.GeoCodeProvider
var AllProviders []*models.GeoCodeProvider
// Did anything change since we last saved the request counts?
var ChangesSinceLastSave bool
// used for sorting providers by lowest next allowed request time - ascending
type ByNextTime []*models.GeoCodeProvider

func (a ByNextTime) Len() int      { return len(a) }
func (a ByNextTime) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByNextTime) Less(i, j int) bool {
	return a[i].NextAllowedRequestTime < a[j].NextAllowedRequestTime
}

// used for sorting providers so the one with more than 1 left request (or 0 requests done) is first.
// But, if we already waited for the complete interval since the server was full it will be moved to top again.
type ByReqsFull []*models.GeoCodeProvider

func (a ByReqsFull) Len() int      { return len(a) }
func (a ByReqsFull) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByReqsFull) Less(i, j int) bool {
	return CheckIfProviderHasRequestsLeft(a[i]) && !CheckIfProviderHasRequestsLeft(a[j])
}

// used for sorting providers by priority - descending
type ByPrio []*models.GeoCodeProvider

func (a ByPrio) Len() int      { return len(a) }
func (a ByPrio) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByPrio) Less(i, j int) bool {
	return a[i].Priority > a[j].Priority
}

// func CheckIfProviderHasRequestsLeft checks if the given provider has more than 1 request remaining, or first interval request is
// more than the IntervalSize ago. (We use nextAllowedRequest as if it was lastRequest, because it is usually not more than
// an hour off)
func CheckIfProviderHasRequestsLeft(provider *models.GeoCodeProvider) bool {
	if provider.FirstIntervalRequest == 0 {
		provider.FirstIntervalRequest = time.Now().UnixNano()
	}
	return provider.MaxRequestsPerInterval-provider.CurIntervalRequests > 1 ||
	provider.CurIntervalRequests == 0 || provider.MaxRequestsPerInterval==0 ||
	provider.FirstIntervalRequest+60*60*24*1000*1000*1000*int64(provider.IntervalSizeInDays) < time.Now().UnixNano()
}
func ReverseGeocode(lat float64, lng float64, dontChain bool, uId string) (res models.Address,usedProvider *models.GeoCodeProvider, err error) {
	success := false
	var availableProviders []*models.GeoCodeProvider
	if dontChain {
		availableProviders = NonChainProviders
	} else {
		availableProviders = ChainProviders
	}
	// First try = the provider with the lowest next allowed request time
	sort.Sort(ByNextTime(availableProviders))
	// First try = the provider with the highest prio
	sort.Sort(ByPrio(availableProviders))
	// But only if there is more than one request left (=try full request servers last to see if the interval-time has passed and new requests are available)
	sort.Sort(ByReqsFull(availableProviders))

	var tempRes models.Address
	for _, v := range availableProviders {
		if v.UsersToReqCount == nil {
			v.UsersToReqCount = make(map[string]int)
		}
		tempRes, err = ReverseGeocodeForProvider(lat, lng, v, uId, false)
		if v.CurIntervalRequests == 1 {
			v.FirstIntervalRequest = time.Now().UnixNano()
		}
		if err != nil {
			if err == ErrNeedFixBeforeRetry {
				dbg.E(TAG, "Error needing fix for geocode provider %s %s (type %d): ", v.Uri, v.Name, v.Type, err)
				v.NextAllowedRequestTime = time.Now().UnixNano() + 60*60*1000*1000*1000

			} else if err == ErrSkipProvider {
				err = nil
				continue
			} else if err == ErrEmptyResult {
				dbg.W(TAG,"Geocoder returned 0 results")
				if v.Type ==2 { // our chain providers already tried all geocoding providers - no sense in trying another
					break;
				}
				continue;
			} else {
				dbg.E(TAG, "Error for geocode provider %s %s (type %d): ", v.Uri, v.Name, v.Type, err)
				v.NextAllowedRequestTime = time.Now().UnixNano() + 5*1000*1000*1000
			}
			continue
		}

		usedProvider = v
		dbg.I(TAG, "Provider %s %s (type %d) has used %d of %d requests", v.Uri, v.Name, v.Type, v.CurIntervalRequests, v.MaxRequestsPerInterval)
		success = true
		if tempRes.HouseNumber == "" || tempRes.Street == "" || tempRes.City == "" {
			if res.City == "" && tempRes.City !=""{
				res = tempRes
			} else if res.Street == "" && tempRes.Street !=""{
				res = tempRes
			}
			if v.Type !=2 { //2=chain provider = last where we could get a better result
				// see if we can find anything better with another provider
				continue
			}
		} else {
			res = tempRes
		}

		break

	}
	if !success {
		if err != ErrEmptyResult {
			err = ErrNoRequestsLeft
		}
	} else {
		err = nil
	}
	RecalcRequestCounts(dontChain)

	return
}


func Geocode(s string, dontChain bool, uId string) (res models.Address,usedProvider *models.GeoCodeProvider, err error) {
	success := false
	var availableProviders []*models.GeoCodeProvider
	if dontChain {
		availableProviders = NonChainProviders
	} else {
		availableProviders = ChainProviders
	}
	// First try = the request with the lowest next allowed request time
	sort.Sort(ByNextTime(availableProviders))
	// First try = the provider with the highest prio
	sort.Sort(ByPrio(availableProviders))
	// But only if there is more than one request left (=try full request servers last to see if the interval-time has passed and new requests are available)
	sort.Sort(ByReqsFull(availableProviders))

	dbg.D(TAG, "Sorted providers : ")
	for _, v := range availableProviders {
		dbg.D(TAG, "Provider : %+v", *v)
	}
	var tempRes models.Address
	for _, v := range availableProviders {
		if v.UsersToReqCount == nil {
			v.UsersToReqCount = make(map[string]int)
		}
		tempRes, err = GeocodeForProvider(s, v, uId, false)
		if v.CurIntervalRequests == 1 {
			v.FirstIntervalRequest = time.Now().UnixNano()
		}
		if err != nil {
			if err == ErrNeedFixBeforeRetry {
				dbg.E(TAG, "Error needing fix for geocode provider %s %s (type %d): ", v.Uri, v.Name, v.Type, err)
				v.NextAllowedRequestTime = time.Now().UnixNano() + 60*60*1000*1000*1000

			} else if err == ErrSkipProvider {
				err = nil
				continue
			} else if err == ErrEmptyResult {
				dbg.W(TAG,"Geocoder returned 0 results")
				if v.Type ==2 { // our chain providers already tried all geocoding providers - no sense in trying another
					break;
				}
				continue;
			}  else {
				dbg.E(TAG, "Error for geocode provider %s %s (type %d): ", v.Uri, v.Name, v.Type, err)
				v.NextAllowedRequestTime = time.Now().UnixNano() + 60*1000*1000*1000
			}
			continue
		}

		usedProvider = v
		dbg.I(TAG, "Provider %s %s (type %d) has used %d of %d requests", v.Uri, v.Name, v.Type, v.CurIntervalRequests, v.MaxRequestsPerInterval)
		success = true
		if tempRes.HouseNumber == "" || tempRes.Street == "" || tempRes.City == "" {
			if res.City == "" && tempRes.City !=""{
				res = tempRes
			}else if res.Street == "" && tempRes.Street !=""{
				res = tempRes
			}
			if v.Type !=2 { //2=chain provider = last where we could get a better result
				// see if we can find anything better with another provider
				continue
			}
		} else {
			res = tempRes
		}

		break

	}
	if !success {
		if err != ErrEmptyResult {
			err = ErrNoRequestsLeft
		}
	} else {
		err = nil
	}
	RecalcRequestCounts(dontChain)

	return
}

func RecalcRequestCounts(dontChain bool) {
	var maxRequestsPerDay int
	var maxRequestsPerUser int
	var curDailyRequestsUsed int
	var curRequestsByUserUsed = make(map[string]int)
	var availableProviders []*models.GeoCodeProvider
	if dontChain {
		availableProviders = NonChainProviders
	} else {
		availableProviders = ChainProviders
	}
	for _, v := range availableProviders {
		maxRequestsPerDay += v.MaxRequestsPerInterval / v.IntervalSizeInDays
		maxRequestsPerUser += v.MaxRequestsPerUserAndDay
		curDailyRequestsUsed += v.CurIntervalRequests
		for uId, cnt := range v.UsersToReqCount {
			curRequestsByUserUsed[uId] += cnt
		}
	}

	MaxRequestsPerDay = maxRequestsPerDay
	MaxRequestsPerUser = maxRequestsPerUser
	CurDailyRequestsUsed = curDailyRequestsUsed
	CurRequestsByUserUsed = curRequestsByUserUsed
	dbg.I(TAG, " Recalced MaxRequestsPerDay %d\r\n MaxRequestsPerUser %d\r\n CurDailyRequestsUsed %d\r\n CurRequestsByUserUsed %+v",
		MaxRequestsPerDay, MaxRequestsPerUser, CurDailyRequestsUsed, CurRequestsByUserUsed)
}

func CheckIfProviderAvailable(provider *models.GeoCodeProvider,uId string) (err error) {
	dbg.I(TAG, "Current provider : %+v", *provider)
	if provider.CurIntervalRequests == 0 || provider.FirstIntervalRequest + 24*60*60*1000*1000*1000*int64(provider.IntervalSizeInDays) < time.Now().UnixNano() {
		provider.UsersToReqCount = make(map[string]int)
		provider.CurIntervalRequests = 0
	}
	if !CheckIfProviderHasRequestsLeft(provider) {
		// we used up our daily contingent
		provider.NextAllowedRequestTime = provider.FirstIntervalRequest + 24*60*60*1000*1000*1000*int64(provider.IntervalSizeInDays)
		if provider.NextAllowedRequestTime > time.Now().UnixNano() {
			dbg.I(TAG, "Geocoding contingent for provider %s %s (type %d) used up - skipping this provider", provider.Uri, provider.Name, provider.Type)
			err = ErrSkipProvider
			return
		}
	}
	if provider.UsersToReqCount[uId] >= provider.MaxRequestsPerUserAndDay && provider.MaxRequestsPerUserAndDay != 0 {
		dbg.I(TAG, "Geocoding contingent for this user & provider %s %s (type %d) used up - skipping this provider", provider.Uri, provider.Name, provider.Type)
		err = ErrSkipProvider
		return
	}
	if provider.NextAllowedRequestTime > time.Now().UnixNano() {
		dbg.I(TAG, "We are before next allowed request time for this geocoder")
		diff := provider.NextAllowedRequestTime - time.Now().UnixNano()
		if diff < 1*1000*1000*1000 {
			dbg.I(TAG, "Waiting, because next allowed time is less than 1 second from now")
			time.Sleep(time.Duration(diff) * time.Nanosecond)
		} else {
			dbg.I(TAG, "Skip this provider")
			err = ErrSkipProvider
			return
		}
	}
	return
}

func ReverseGeocodeForProvider(lat float64, lng float64, provider *models.GeoCodeProvider, userId string, dontChain bool) (res models.Address, err error) {
	err = CheckIfProviderAvailable(provider,userId)
	ChangesSinceLastSave = true
	if err != nil {
		return
	}
	uri := provider.Uri
	switch provider.Type {
	case 1: // geocode.farm
		{
			uri = uri + fmt.Sprintf("/reverse/?lat=%f&lon=%f&lang=en", lat, lng)
		}
	case 2: // Chain
		{
			uri = uri + fmt.Sprintf("/reverse/%s/bla/blub/%f/%f?dontChain=1", userId, lat, lng)
		}
	case 3:  // Tomtom
		{
			uri = uri + fmt.Sprintf("/reverseGeocode/%f,%f.JSON?key=%s", lat,lng,provider.Key1)
		}
	case 4:  // OpenCage
		{
			uri = uri + fmt.Sprintf("&q=%f,%f&key=%s", lat,lng,provider.Key1)
		}
	}

	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		dbg.E(TAG, "Error initializing httpRequest : ", err)
		return
	}
	/* Get Details */
	var resp *http.Response
	provider.LastRequestTime = time.Now().UnixNano()
	provider.NextAllowedRequestTime = provider.LastRequestTime + provider.TimeBetweenRequests
	ChangesSinceLastSave = true
	if Debug {
		dbg.I(TAG, "Sending request with uri : %s", uri)
	}
	if provider.Type==3 { // TomTom does not report back current daily count
		if provider.FirstIntervalRequest + 60*60*24*1000*1000*1000<time.Now().UnixNano() {
			provider.CurIntervalRequests = 0
			provider.UsersToReqCount = make(map[string]int)
			provider.FirstIntervalRequest = time.Now().UnixNano()
		}
		provider.CurIntervalRequests++
	}
	resp, err = client.Do(req)
	if err != nil {
		dbg.E(TAG, "Error executing reverse geocode request: %s", err)
		return
	}
	var _body []byte
	_body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		if err != nil {
			dbg.E(TAG, "Error reading reverse geocode response: %s", err)
			FillUnknownAddress(&res)
			return res, ErrNeedFixBeforeRetry
		}
	}
	err = FillAddrAndNextTimeFromResp(_body,provider,&res, userId,true)
	if provider.Type!=2 { // our chain provider returns the requests used for this user, for others we need to keep track ourselfs.
		provider.UsersToReqCount[userId] = provider.UsersToReqCount[userId] + 1
	}
	ChangesSinceLastSave = true
	return
}

var OpenCageRegExp *regexp.Regexp

func GeocodeForProvider(s string, provider *models.GeoCodeProvider, userId string, dontChain bool) (res models.Address, err error) {
	if OpenCageRegExp == nil {
		// replace Walterstal 101 09599 Freiberg with Walterstal 101, 09599 Freiberg
		OpenCageRegExp = regexp.MustCompile("([0-9][A-Z]?)\\ ([0-9]{4,5})\\ (\\w)")
	}
	err = CheckIfProviderAvailable(provider,userId)
	ChangesSinceLastSave = true
	if err != nil {
		return
	}
	uri := provider.Uri
	switch provider.Type {
	case 1: // geocode.farm
		{
			uri = uri + fmt.Sprintf("/forward/?addr=%s&lang=en=1",url.QueryEscape(s))
		}
	case 2: // Chain
		{
			uri = uri + fmt.Sprintf("/forward/%s/b/b/%s?dontChain=1", userId, url.QueryEscape(s))
		}
	case 3:  // Tomtom
		{

			uri = uri + fmt.Sprintf("/geocode/%s.JSON?key=%s", url.QueryEscape(s),provider.Key1)
		}
	case 4:  // OpenCage
		{
			// replace Walterstal 101 09599 Freiberg with Walterstal 101, 09599 Freiberg
			s = OpenCageRegExp.ReplaceAllString(s,"$1, $2 $3")
			uri = uri + fmt.Sprintf("&q=%s&key=%s", url.QueryEscape(s),provider.Key1)
		}
	}
	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		dbg.E(TAG, "Error initializing httpRequest : ", err)
		return
	}
	/* Get Details */
	var resp *http.Response
	provider.LastRequestTime = time.Now().UnixNano()
	provider.NextAllowedRequestTime = provider.LastRequestTime + provider.TimeBetweenRequests
	ChangesSinceLastSave = true
	if Debug {
		dbg.I(TAG, "Sending request with uri : %s", uri)
	}
	resp, err = client.Do(req)
	if err != nil {
		dbg.E(TAG, "Error executing forward geocode request: %s", err)
		return
	}
	var _body []byte
	_body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		if err != nil {
			dbg.E(TAG, "Error reading forward geocode response: %s", err)
			FillUnknownAddress(&res)
			return res, ErrNeedFixBeforeRetry
		}
	}
	err = FillAddrAndNextTimeFromResp(_body,provider,&res, userId,false)
	if provider.Type!=2 {
		// our chain provider returns the requests used for this user, for others we need to keep track ourselfs.
		provider.UsersToReqCount[userId] = provider.UsersToReqCount[userId] + 1
	}
	ChangesSinceLastSave = true
	return
}

func FillAddrAndNextTimeFromResp(_body []byte,provider *models.GeoCodeProvider,res *models.Address,uId string, isReverse bool) (err error){
	switch provider.Type {
	case 1: // GeoFarm
		{
			err = FillAddrFromGeoFarmResp(_body, provider, res)
		}
	case 2: // Chain
		{
			err = FillAddrFromChainResp(_body, provider, res, uId)
		}
	case 3: // TomTom
		{
			if isReverse {
				err = FillAddrFromTomTomReverseResp(_body, provider, res)

			} else {
				err = FillAddrFromTomTomForwardResp(_body, provider, res)
			}
		}
	case 4: // OpenCage
		{
			err = FillAddrFromOpenCageResp(_body, provider, res)
		}
	default:
		err = ErrProviderNotSupported
	}

	if provider.MaxRequestsPerInterval-provider.CurIntervalRequests < 0 { // usage limit exceeded - wait 10 minutes before next request
		provider.NextAllowedRequestTime = time.Now().UnixNano() + 10*60*1000*1000*1000
	}
	if err != nil {
		dbg.W(TAG, "Error filling address : ", err)
	}
	return
}
func FillUnknownAddress(add *models.Address) {
	add.Street = "Unbekannt"
	add.Postal = ""
	add.City = "Unbekannt"
	add.HouseNumber = ""
	return
}
func FillAddrFromGeoFarmResp(resp []byte, provider *models.GeoCodeProvider, addr *models.Address) (err error) {
	if addr == nil {
		dbg.E(TAG, "Error : Got nil address to fil")
		return errors.New("Address object is nil.")
	}

	var r models.GeoCodeFarmResult
	res := models.GeoCodeFarmResp{}
	err = json.Unmarshal(resp, &res)
	if err != nil {
		dbg.E(TAG, "Error processing GeoCodeFarmResponse : ", err)
		if Debug {
			dbg.I(TAG, "Response : ", string(resp))
		}
	}
	if Debug {
		dbg.I(TAG, "Parsed result : %+v \r\n from resp %s", res.GeocodingResults, string(resp))
	}
	if len(res.GeocodingResults.Results) > 0 {
		if len(res.GeocodingResults.Results) == 1 {
			r = res.GeocodingResults.Results[0]
		} else {
			for _,v := range res.GeocodingResults.Results {
				if r.FormattedAddress =="" {
					r = v
				} else if r.Address.Locality=="" && v.Address.Locality!="" {
					r = v
				} else if r.Address.Postal=="" && v.Address.Postal!="" && v.Address.Locality!=""{
					r = v
				} else if r.Address.StreetName=="" && v.Address.StreetName!="" && v.Address.Postal!="" && v.Address.Locality!=""{
					r = v
				} else if r.Address.StreetNumber=="" && v.Address.StreetNumber!="" && v.Address.StreetName!="" && v.Address.Postal!="" && v.Address.Locality!=""{
					r = v
				}
				if r.Address.StreetNumber != "" {
					break;
				}
			}
		}
	}
	acc := res.GeocodingResults.Account

	if acc.UsageLimit != "" {
		var ul int64
		var us int64
		ul, err = strconv.ParseInt(acc.UsageLimit, 10, 64)
		if err != nil {
			dbg.W(TAG, "Could not parse usage limit : ", acc.UsageLimit, err)
			err = nil
		} else {
			provider.MaxRequestsPerInterval = int(ul)
			us, err = strconv.ParseInt(acc.UsedToday, 10, 64)
			if err != nil {
				dbg.W(TAG, "Could not parse used today : ", acc.UsedToday, err)
				err = nil
			} else {
				provider.CurIntervalRequests = int(us)
			}
		}
	} else {
		dbg.W(TAG, "No Account in GeoCodeFarmResponse?")
	}

	if r.FormattedAddress != "" {

		a := r.Address
		addr.HouseNumber = a.StreetNumber
		addr.City = a.Locality
		addr.Street = a.StreetName
		addr.Postal = a.Postal
		addr.Lat, _ = strconv.ParseFloat(r.Coordinates.Latitude, 64)
		addr.Lng, _ = strconv.ParseFloat(r.Coordinates.Longitude, 64)
		addr.Country = a.Country
		addr.Title = r.FormattedAddress
		if Debug {
			dbg.I(TAG, "Got address \r\n %+v \r\n out of result \r\n %+v \r\n with address \r\n %+v", addr, r, a)
		}
		return
	} else {
		FillUnknownAddress(addr)
		return ErrEmptyResult
	}

}

func FillAddrFromTomTomForwardResp(resp []byte, provider *models.GeoCodeProvider, addr *models.Address) (err error) {
	if addr == nil {
		dbg.E(TAG, "Error : Got nil address to fil")
		return errors.New("Address object is nil.")
	}

	var r models.TomTomForwardResult
	res := models.TomTomForwardResp{}
	err = json.Unmarshal(resp, &res)
	if err != nil {
		dbg.E(TAG, "Error processing TomTomForwardResponse : ", err)
		if Debug {
			dbg.I(TAG, "Response : ", string(resp))
		}
	}
	if Debug {
		dbg.I(TAG, "Parsed result : %+v \r\n from resp %s", res, string(resp))
	}
	if len(res.Results) > 0 {
		if len(res.Results) == 1 {
			r = res.Results[0]
		} else {
			for _,v := range res.Results {
				if CompareTomTomAddress(&v.Address,&r.Address) {
					r = v
				}
				if r.Address.StreetNumber != "" || r.Address.BuildingNumber!="" {
					break;
				}
			}
		}
	}

	if r.Address.FreeFormAddress != "" {
		FillAddrFromTomTomAddress(&r.Address,addr)
		addr.Lat = r.Position.Lat
		addr.Lng = r.Position.Lon

		if Debug {
			dbg.I(TAG, "Got address \r\n %+v \r\n out of result \r\n %+v \r\n with address \r\n %+v", addr, r, r.Address)
		}
		return
	} else {
		FillUnknownAddress(addr)
		return ErrEmptyResult
	}

}

func FillAddrFromTomTomReverseResp(resp []byte, provider *models.GeoCodeProvider, addr *models.Address) (err error) {
	if addr == nil {
		dbg.E(TAG, "Error : Got nil address to fil")
		return errors.New("Address object is nil.")
	}

	var res models.TomTomReverseResp
	var r models.TomTomReverseResult
	err = json.Unmarshal(resp, &res)
	if err != nil {
		dbg.E(TAG, "Error processing TomTomReverseResponse : ", err)
		if Debug {
			dbg.I(TAG, "Response : ", string(resp))
		}
	}
	if Debug {
		dbg.I(TAG, "Parsed result : %+v \r\n from resp %s", res, string(resp))
	}
	if len(res.Addresses) > 0 {
		if len(res.Addresses) == 1 {
			r = res.Addresses[0]
		} else {
			for _,v := range res.Addresses {
				if CompareTomTomAddress(&v.Address,&r.Address) {
					r = v
				}
				if r.Address.StreetNumber != "" || r.Address.BuildingNumber!="" {
					break;
				}
			}
		}
	}

	if r.Address.FreeFormAddress != "" {
		FillAddrFromTomTomAddress(&r.Address,addr)
		splitted := strings.Split(r.Position,",")
		if len(splitted)==2 {
			addr.Lat, _ = strconv.ParseFloat(splitted[0],64)
			addr.Lng, _ = strconv.ParseFloat(splitted[1],64)
		}

		if Debug {
			dbg.I(TAG, "Got address \r\n %+v \r\n out of result \r\n %+v \r\n with address \r\n %+v", addr, r, r.Address)
		}
		return
	} else {
		FillUnknownAddress(addr)
		return ErrEmptyResult
	}

}

func FillAddrFromTomTomAddress (a *models.TomTomAddress,b *models.Address) {
	if a.StreetNumber != "" {
		b.HouseNumber = a.StreetNumber
	} else {
		b.HouseNumber = a.BuildingNumber
	}
	b.City = a.Municipality
	if a.Street!="" {
		b.Street = a.Street
	} else {
		b.Street = a.StreetName
	}
	// Chemnitzer Straße, Dampfbahn-Route Sachsen
	commaIdx := strings.Index(string(b.Street), ",")
	if commaIdx > 0 {
		b.Street = string(b.Street)[0:commaIdx]
	}
	b.Postal = a.PostalCode
	b.Country = a.CountryCode
	b.Title = a.FreeFormAddress
}

func FillAddrFromOpenCageAddress (r *models.OpenCageResult,b *models.Address) {
	a := r.Components
	b.HouseNumber = a.HouseNumber
	if a.City!="" {
		b.City = a.City
	} else {
		b.City = a.Town
	}
	if a.Road == "" {
		b.Street = a.Footway
	} else {
		b.Street = a.Road
	}
	// Chemnitzer Straße, Dampfbahn-Route Sachsen
	commaIdx := strings.Index(string(b.Street), ",")
	if commaIdx > 0 {
		b.Street = string(b.Street)[0:commaIdx]
	}
	b.Postal = a.PostCode
	b.Country = a.Country
	b.Title = r.Formatted
	b.Lat = r.Geometry.Lat
	b.Lng = r.Geometry.Lng
	b.Accuracy = fmt.Sprintf("%d",r.Confidence)
	b.Fuel = a.Fuel
}
// returns true if a is better then b
func CompareTomTomAddress(a *models.TomTomAddress, b *models.TomTomAddress) (bool) {
	if a.FreeFormAddress =="" && b.FreeFormAddress!=""{
		return false
	} else if a.Municipality=="" && b.Municipality!="" {
		return false
	} else if a.PostalCode=="" && b.PostalCode!="" && b.Municipality!=""{
		return false
	} else if a.StreetName=="" && a.Street=="" &&
	(b.StreetName!="" || b.Street!="") &&
	b.PostalCode!="" && b.Municipality!=""{
		return false
	} else if a.StreetNumber=="" && a.BuildingNumber=="" && (b.StreetNumber!="" || b.BuildingNumber!="" )&&
	(b.StreetName!="" || b.Street!="") &&
	b.PostalCode!="" && b.Municipality!="" {
		return false
	}
	return true
}
func FillAddrFromChainResp(resp []byte, provider *models.GeoCodeProvider, addr *models.Address, uId string) (err error) {
	if addr == nil {
		dbg.E(TAG, "Error : Got nil address to fil")
		return errors.New("Address object is nil.")
	}

	var r models.GeoResp
	err = json.Unmarshal(resp, &r)
	if err != nil {
		dbg.E(TAG, "Error processing Chain Resp : ", err)
		if Debug {
			dbg.I(TAG, "Response : ", string(resp))
		}
	}
	*addr = r.Address
	provider.MaxRequestsPerUserAndDay = r.MaxRequestsPerUser
	provider.MaxRequestsPerInterval = r.MaxRequestsPerDay
	provider.CurIntervalRequests = r.CurDailyRequestsUsed
	provider.UsersToReqCount[uId] = r.CurUserRequestsUsed
	return
}

func FillAddrFromOpenCageResp(resp []byte, provider *models.GeoCodeProvider, addr *models.Address) (err error) {
	if addr == nil {
		dbg.E(TAG, "Error : Got nil address to fil")
		return errors.New("Address object is nil.")
	}

	var r models.OpenCageResult
	res := models.OpenCageResponse{}
	err = json.Unmarshal(resp, &res)
	if err != nil {
		dbg.E(TAG, "Error processing OpenCageResponse : ", err)
		if Debug {
			dbg.I(TAG, "Response : ", string(resp))
		}
	}
	if Debug {
		dbg.I(TAG, "Parsed result : %+v \r\n from resp %s", res, string(resp))
	}
	provider.FirstIntervalRequest = int64(res.Rate.Reset)*1000*1000*1000-60*60*24*1000*1000*1000 // 24 hours before interval reset = first request
	provider.CurIntervalRequests = res.Rate.Limit-res.Rate.Remaining
	provider.MaxRequestsPerInterval = res.Rate.Limit
	if len(res.Results) > 0 {
		if len(res.Results) == 1 {
			r = res.Results[0]
		} else {
			for _,v := range res.Results {
				if r.Formatted =="" && v.Formatted!=""{
					r = v
				} else if r.Components.Town=="" && r.Components.City == "" && (v.Components.Town!="" || v.Components.City!="") {
					r = v
				} else if r.Components.PostCode=="" && v.Components.PostCode!="" && (v.Components.Town!="" || v.Components.City!="") {
					r = v
				} else if r.Components.Road=="" && r.Components.Footway=="" && (v.Components.Road!="" || v.Components.Footway!="") &&
				v.Components.PostCode!="" && (v.Components.Town!="" || v.Components.City!=""){
					r = v
				} else if r.Components.HouseNumber=="" && v.Components.HouseNumber!="" &&
				(v.Components.Road!="" || v.Components.Footway!="") &&
				v.Components.PostCode!="" && (v.Components.Town!="" || v.Components.City!="") {
					r = v
				} else if r.Components.Fuel=="" && v.Components.Fuel!="" &&
				v.Components.HouseNumber!="" && (v.Components.Road!="" || v.Components.Footway!="") &&
				v.Components.PostCode!="" && (v.Components.Town!="" || v.Components.City!="") {
					r = v
				}
				if r.Components.HouseNumber != "" && r.Components.Fuel != "" && (r.Components.Road!="" || r.Components.Footway!="") {
					break;
				}
			}
		}
	}

	if r.Formatted != "" {
		FillAddrFromOpenCageAddress(&r,addr)
		if Debug {
			dbg.I(TAG, "Got address \r\n %+v \r\n out of result \r\n %+v \r\n with address \r\n %+v", addr, r, r.Components)
		}
		return
	} else {
		FillUnknownAddress(addr)
		return ErrEmptyResult
	}

}
func ParseProviders(jsonb []byte) (err error) {
	if len(AllProviders) != 0 {
		SaveProviders(true)
	}
	servers := []*models.GeoCodeProvider{}
	err = json.Unmarshal(jsonb, &servers)
	if err != nil {
		dbg.E(TAG, "Error parsing chained servers :( ", err)
		return
	}
	ChainProviders = make ([]*models.GeoCodeProvider,0)
	NonChainProviders = make ([]*models.GeoCodeProvider,0)
	AllProviders = make ([]*models.GeoCodeProvider,0)
	savedProviders := make([]*models.GeoCodeProvider,0)
	provNameToSave := make(map[string]*models.GeoCodeProvider)
	if _, _err := os.Stat("AutoSavedProviders.json"); _err == nil {

		var b []byte
		b, err = ioutil.ReadFile("AutoSavedProviders.json")
		if err != nil {
			dbg.E(TAG, "Error reading previous provider data : ")
			return
		}
		err = json.Unmarshal(b, &savedProviders)
		if err != nil {
			dbg.E(TAG, "Error parsing previous provider data : ")
			return
		}

		for _, v := range savedProviders {
			if provNameToSave[v.Name] != nil {
				dbg.E(TAG, "Multiple providers with same name not allowed! Please correct this in Providers.json AND AutoSavedProviders.json ")
				return errors.New("Multiple providers with same name not allowed! Please correct this in Providers.json AND AutoSavedProviders.json")
			}
			provNameToSave[v.Name] = v
		}
	}
	for _, v := range servers {
		if provNameToSave[v.Name] != nil {
			p := provNameToSave[v.Name]
			v.CurIntervalRequests = p.CurIntervalRequests
			v.FirstIntervalRequest = p.FirstIntervalRequest
			v.LastRequestTime = p.LastRequestTime
			v.NextAllowedRequestTime = p.NextAllowedRequestTime
			v.UsersToReqCount = p.UsersToReqCount
			dbg.WTF(TAG,"Updated provider from AutoSavedProviders.json - Result : %+v",v)
		}
		AllProviders = append(AllProviders, v)
		if !v.Disabled {
			NonChainProviders = append(NonChainProviders,v)
			ChainProviders = append(ChainProviders, v)
			if !v.ChainingForbidden { // Yes, this seems counter-intuitive - but if dontChain = false, we are the root-server
				// and need to ask this provider - otherwise we don't.
				NonChainProviders = append(NonChainProviders,v)
			}
		}
	}
	return
}

func SaveProviders(force bool) {
	if ChangesSinceLastSave || force {
		data, err := json.Marshal(AllProviders)
		if err != nil {
			dbg.E(TAG,"Unable to marshal providers : ", err)
		}
		err = ioutil.WriteFile("AutoSavedProviders.json",data,os.FileMode(0777))
		if err != nil {
			dbg.E(TAG,"Unable to autosave providers : ", err)
		}
	}
}