package json

import (
	"encoding/json"
	"github.com/Compufreak345/dbg"
	"github.com/OpenDriversLog/odl-geocoder/models"
	"github.com/OpenDriversLog/odl-geocoder/utils"
	"strconv"
)

const TAG = "ogc/json.go"

func GetJsonReverseGeoCode(sLat string, sLng string, reqId string, dontChain bool, uId string) (output []byte, err error) {
	var res models.GeoResp
	if sLat == "" || sLng == "" {
		output, err = json.Marshal(GetErrorGeoCodeResponse("No lat/lng provided", reqId))
		return
	} else {
		lat, _err := strconv.ParseFloat(sLat, 64)
		if _err != nil {
			output, err = json.Marshal(GetErrorGeoCodeResponse("Latitude not parsable", reqId))
			return
		}
		lng, _err := strconv.ParseFloat(sLng, 64)
		if _err != nil {
			output, err = json.Marshal(GetErrorGeoCodeResponse("Longitude not parsable", reqId))
			return
		}
		add, prov, _err := utils.ReverseGeocode(lat, lng, dontChain, uId)
		if _err != nil {
			if _err == utils.ErrNoRequestsLeft {
				dbg.W(TAG, "No requests left :(")
				output, err = json.Marshal(GetErrorGeoCodeResponse("No working geocoders left :(", reqId))
				return

			} else if _err == utils.ErrEmptyResult {
				dbg.W(TAG,"No geocoding result found...")
			} else {
				dbg.E(TAG, "Error reverse geocoding : ", _err)
				output, err = json.Marshal(GetErrorGeoCodeResponse("Error reversing", reqId))
				return
			}
		}
		res.Address = add
		res.ReqId = reqId
		res.CurUserRequestsUsed = utils.CurRequestsByUserUsed[uId]
		res.CurDailyRequestsUsed = utils.CurDailyRequestsUsed
		res.MaxRequestsPerUser = utils.MaxRequestsPerUser
		res.MaxRequestsPerDay = utils.MaxRequestsPerDay
		if prov!=nil {
			res.Provider = prov.Name
		}
	}
	output, err = json.Marshal(res)
	if err != nil {
		dbg.E(TAG, "Error marshaling : ", err)
	}
	return
}

func GetJsonGeoCode(s string, reqId string, dontChain bool, uId string) (output []byte, err error) {
	var res models.GeoResp
	if s == "" {
		output, err = json.Marshal(GetErrorGeoCodeResponse("No address provided", reqId))
		return
	}

	add, prov, _err := utils.Geocode(s, dontChain, uId)
	if _err != nil {
		if _err == utils.ErrNoRequestsLeft {
			dbg.W(TAG, "No requests left :(")
			output, err = json.Marshal(GetErrorGeoCodeResponse("No working geocoders left :(", reqId))
			return
		} else if _err == utils.ErrEmptyResult {
			dbg.W(TAG,"No geocoding result found...")
			err = nil
		} else {
			dbg.E(TAG, "Error forward geocoding : ", _err)
			output, err = json.Marshal(GetErrorGeoCodeResponse("Error reversing", reqId))
			return
		}
	}
	res.Address = add
	res.ReqId = reqId
	res.CurUserRequestsUsed = utils.CurRequestsByUserUsed[uId]
	res.CurDailyRequestsUsed = utils.CurDailyRequestsUsed
	res.MaxRequestsPerUser = utils.MaxRequestsPerUser
	res.MaxRequestsPerDay = utils.MaxRequestsPerDay
	if prov!=nil {
		res.Provider = prov.Name
	}
	output, err = json.Marshal(res)
	if err != nil {
		dbg.E(TAG, "Error marshaling : ", err)
	}
	return
}

func GetErrorGeoCodeResponse(msg string, reqId string) (res models.GeoResp) {
	res = models.GeoResp{
		ReqId: reqId,
		Error: msg,
	}
	return
}
