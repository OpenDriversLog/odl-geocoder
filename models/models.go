package models

/*
Models for odl-geocoder - make sure to keep in sync with goodl-lib/models/odl-geocode
Having this redundancy because importing odl-geocoder in goodl-lib is quite annoying.
 */

type GeoResp struct {
	ReqId                string
	Lat                  float64
	Lng                  float64
	Address              Address
	MaxRequestsPerDay    int
	MaxRequestsPerUser   int
	CurDailyRequestsUsed int
	CurUserRequestsUsed  int
	Error                string
	Provider string
}

type Address struct {
	Lat         float64
	Lng         float64
	Street      string
	Postal      string
	City        string
	Additional1 string
	Additional2 string
	HouseNumber string
	Title       string
	Fuel        string
	Accuracy    string
	Country     string
}

type GeoCodeProvider struct {
	Type                     int64          // needs to be set up manually - 1=geocode.farm, 2 = Chained odl-geocoder
	Name                     string         // needs to be set up manually
	Key1                     string         // currently not used
	Key2                     string         // currently not used
	Key3                     string         // currently not used
	Key4                     string         // currently not used
	Uri                      string         // needs to be set up manually
	MaxRequestsPerInterval   int            // usually gets filled automatically, as the provider returns the limits on a request
	CurIntervalRequests      int            // usually gets filled automatically, as the provider returns the limits on a request
	MaxRequestsPerUserAndDay int            // for chained geocoder gets set automatically, manually for geocode.farm
	IntervalSizeInDays       int            // needs to be set up manually
	LastRequestTime          int64          // UnixNano of last request
	NextAllowedRequestTime   int64          // UnixNano when next request is allowed
	TimeBetweenRequests      int64          // time in nanoseconds that needs to be wait between requests
	UsersToReqCount          map[string]int // not reboot-save.
	Disabled                 bool           // set true if this is your own IP
	ChainingForbidden	bool
	Priority		int		// higher is better
	FirstIntervalRequest	int64		// time when the current request interval started
}

type GeoCodeFarmResp struct {
	GeocodingResults GeoCodeFarmGeocodingResults `json:"geocoding_results"`
}

type GeoCodeFarmGeocodingResults struct {
	Copyright  GeoCodeFarmCopyright  `json:"LEGAL_COPYRIGHT"`
	Status     GeoCodeFarmStatus     `json:"STATUS"`
	Account    GeoCodeFarmAccount    `json:"ACCOUNT"`
	Results    []GeoCodeFarmResult   `json:"RESULTS"`
	Statistics GeocodeFarmStatistics `json:"STATISTICS"`
}

type GeoCodeFarmCopyright struct {
	CopyrightNotice string `json:"copyright_notice"`
	CopyrightLogo   string `json:"copyright_logo"`
	ToS             string `json:"terms_of_service"`
	PrivacyPolicy   string `json:"privacy_policy"`
}

type GeoCodeFarmStatus struct {
	Access      string `json:"access"`
	Status      string `json:"status"`
	LatProvided string `json:"latitude_provided"`
	LonProvided string `json:"longitude_provided"`
	ResultCount int64  `json:"result_count"`
}
type GeoCodeFarmAccount struct {
	Ip         string `json:"ip_address"`
	License    string `json:"distribution_license"`
	UsageLimit string `json:"usage_limit"`
	UsedToday  string `json:"used_today"`
	UsedTotal  string `json:"used_total"`
	FirstUsed  string `json:"first_used"`
}

type GeoCodeFarmResult struct {
	ResultNumber     int64                      `json:"result_number"`
	FormattedAddress string                     `json:"formatted_address"`
	Accuracy         string                     `json:"Accuracy"`
	Address          GeocodeFarmAddress         `json:"ADDRESS"`
	LocationDetails  GeocodeFarmLocationDetails `json:"LOCATION_DETAILS"`
	Coordinates      GeoCodeFarmCoordinates     `json:"COORDINATES"`
	Boundaries       GeoCodeFarmBoundaries      `json:"BOUNDARIES"`
}

type GeocodeFarmAddress struct {
	StreetNumber string `json:"street_number"`
	StreetName   string `json:"street_name"`
	Locality     string `json:"locality"`
	Admin1       string `json:"admin_1"`
	Admin2       string `json:"admin_2"`
	Postal       string `json:"postal_code"`
	Country      string `json:"country"`
}

type GeocodeFarmLocationDetails struct {
	Elevation     string `json:"elevation"`
	TimeZoneLong  string `json:"timezone_long"`
	TimeZoneShort string `json:"timezone_short"`
}

type GeoCodeFarmCoordinates struct {
	Latitude  string `json:"latitude"`
	Longitude string `json:"longitude"`
}

type GeoCodeFarmBoundaries struct {
	NorthEastLatitude  string `json:"northeast_latitude"`
	NorthEastLongitude string `json:"northeast_longitude"`
	SouthWestLatitude  string `json:"northwest_latitude"`
	SouthWestLongitude string `json:"southwest_longitude"`
}

type GeocodeFarmStatistics struct {
	HttpsSSL string `json:"https_ssl"`
}

type TomTomForwardResp struct {
	Summary TomTomSummary `json:"summary"`
	Results []TomTomForwardResult `json:"results"`
}

type TomTomReverseResp struct {
	Summary TomTomSummary `json:"summary"`
	Addresses []TomTomReverseResult `json:"addresses"`
}

type TomTomSummary struct {
	Query string `json:"query"`
	QueryType string `json:"queryType"`
	QueryTime int `json:"QueryTime"`
	NumResults int `json:"numResults"`
	TotalResults int `json:"totalResults"`
}


type TomTomReverseResult struct {
	Type string `json:"type"`
	Address TomTomAddress `json:"address"`
	Position string `json:"position"`
	AddressRanges string `json:"addressRanges"`
}

type TomTomForwardResult struct {
	Address TomTomAddress `json:"address"`
	Position TomTomPosition `json:"position"`
	RoadUse []string
}

type TomTomAddress struct {
	StreetNumber string `json:"streetNumber"`
	BuildingNumber string `json:"buildingNumber"` // Reverse only
	StreetName string `json:"streetName"`
	Street string `json:"street"` // Reverse only
	StreetNameAndNumber string `json:"streetNameAndNumber"` // Reverse only
	SpeedLimit string `json:"speedLimit"` // Reverse only
	MunicipalitySubdivision string `json:"municipalitySubdivision"`
	Municipality string `json:"municipality"`
	CountrySecondarySubdivision string `json:"countrySecondarySubdivision"`
	CountryTertiarySubdivision string `json:"countryTertiarySubdivision"`
	CountrySubdivision string `json:"countrySubdivision"`
	PostalCode string `json:"postalCode"`
	CountryCode string `json:"countryCode"`
	FreeFormAddress string `json:"freeformAddress"`
}

type TomTomPosition struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"Lon"`
}

type OpenCageResponse struct {
	Documentation string `json:"Documentation"`
	Licenses []OpenCageLicense `json:"licenses"`
	Rate OpenCageRate `json:"rate"`
	Results []OpenCageResult `json:"results"`
	Status OpenCageStatus `json:"status"`
	StayInformed OpenCageStayInformed `json:"stay_informed"`
	Thanks string `json:"thanks"`
	Timestamp OpenCageTimestamp `json:"timestamp"`
	TotalResults int `json:"total_results"`
}

type OpenCageLicense struct {
	Name string `json:"Name"`
	Url string `json:"Url"`
}

type OpenCageRate struct {
	Limit int `json:"limit"`
	Remaining int `json:"Remaining"`
	Reset int `json:"Reset"`
}

type OpenCageResult struct {
	Bounds OpenCageBounds `json:"bounds"`
	Components OpenCageComponents `json:"components"`
	Confidence int64 `json:"confidence"`
	Formatted string `json:"formatted"`
	Geometry OpenCageGeometry `json:"geometry"`

}

type OpenCageBounds struct {
	NorthEast OpenCageGeometry `json:"northeast"`
	SouthWest OpenCageGeometry `json:"southwest"`
}

type OpenCageComponents struct {
	City string `json:"city"`
	Clothes string `json:"clothes"`
	Country string `json:"country"`
	County string `json:"county"`
	HouseNumber string `json:"house_number"`
	Neighbourhood string `json:"neighbourhood"`
	PostCode string `json:"postcode"`
	Town string `json:"town"`
	CountryCode string `json:"country_code"`
	Road string `json:"road"`
	Footway string `json:"footway"`
	State string `json:"state"`
	Suburb string `json:"suburb"`
	Fuel string `json:"fuel"`

}

type OpenCageGeometry struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

type OpenCageStayInformed struct {
	Blog string `json:"blog"`
	Twitter string `json:"twitter"`
}

type OpenCageTimestamp struct {
	CreatedHttp string `json:"created_http"`
	CreatedUnix int32 `json:"created_unix"`
}
type OpenCageStatus struct {
	Code int `json:"code"`
	Message string `json:"message"`
}