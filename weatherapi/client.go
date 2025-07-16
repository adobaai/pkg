// Package weatherapi provides a client for the WeatherAPI.com API.
package weatherapi

import (
	"context"
	"fmt"
	"net/http"

	"github.com/adobaai/pkg/netz/httpz"
)

const (
	baseURL = "https://api.weatherapi.com/v1"
)

type Client struct {
	key string
	hc  *http.Client
}

func NewClient(key string) *Client {
	return &Client{
		key: key,
		hc:  http.DefaultClient,
	}
}

type query struct {
	City   string
	USZip  string
	AutoIP bool
}

func (q *query) String() string {
	s := ""
	if q.City != "" {
		s = q.City
	} else if q.USZip != "" {
		s = q.USZip
	} else if q.AutoIP {
		s = "auto:ip"
	}
	return s
}

type QueryOption func(*query)

func WithCity(city string) QueryOption {
	return func(q *query) {
		q.City = city
	}
}

func WithUSZip(zip string) QueryOption {
	return func(q *query) {
		q.USZip = zip
	}
}

func WithAutoIP() QueryOption {
	return func(q *query) {
		q.AutoIP = true
	}
}

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type ErrorResp struct {
	StatusCode int   // The HTTP status code.
	Err        Error `json:"error"`
}

func (er *ErrorResp) Error() string {
	return fmt.Sprintf("http (%d): %s (%d)", er.StatusCode, er.Err.Message, er.Err.Code)
}

func (c *Client) GetCurrent(ctx context.Context, opts ...QueryOption) (res *Current, err error) {
	q := &query{}
	for _, opt := range opts {
		opt(q)
	}

	url := baseURL + "/current.json?key=" + c.key + "&q=" + q.String()
	res, er, err := httpz.JSON2[Current, ErrorResp](ctx, c.hc, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if !er.IsZero() {
		er.T.StatusCode = er.StatusCode
		return nil, er.T
	}

	return
}

type Current struct {
	Location Location
	Current  Weather
}

type Location struct {
	Name           string  `json:"name"`
	Region         string  `json:"region"`
	Country        string  `json:"country"`
	Lat            float64 `json:"lat"`
	Lon            float64 `json:"lon"`
	TzID           string  `json:"tz_id"`
	LocaltimeEpoch int     `json:"localtime_epoch"`
	Localtime      string  `json:"localtime"`
}

type Weather struct {
	LastUpdatedEpoch int       `json:"last_updated_epoch"`
	LastUpdated      string    `json:"last_updated"`
	TempC            float64   `json:"temp_c"`
	TempF            float64   `json:"temp_f"`
	IsDay            int       `json:"is_day"`
	Condition        Condition `json:"condition"`
	WindMPH          float64   `json:"wind_mph"`
	WindKPH          float64   `json:"wind_kph"`
	WindDegree       int       `json:"wind_degree"`
	WindDir          string    `json:"wind_dir"`
	PressureMB       float64   `json:"pressure_mb"`
	PressureIN       float64   `json:"pressure_in"`
	PrecipMM         float64   `json:"precip_mm"`
	PrecipIN         float64   `json:"precip_in"`
	Humidity         int       `json:"humidity"`
	Cloud            int       `json:"cloud"`
	FeelslikeC       float64   `json:"feelslike_c"`
	FeelslikeF       float64   `json:"feelslike_f"`
	WindchillC       float64   `json:"windchill_c"`
	WindchillF       float64   `json:"windchill_f"`
	HeatindexC       float64   `json:"heatindex_c"`
	HeatindexF       float64   `json:"heatindex_f"`
	DewpointC        float64   `json:"dewpoint_c"`
	DewpointF        float64   `json:"dewpoint_f"`
	VisKM            float64   `json:"vis_km"`
	VisMiles         float64   `json:"vis_miles"`
	UV               float64   `json:"uv"`
	GustMPH          float64   `json:"gust_mph"`
	GustKPH          float64   `json:"gust_kph"`
}

type Condition struct {
	Text string `json:"text"`
	Icon string `json:"icon"`
	Code int    `json:"code"`
}

/* Example error:
{
	"error": {
		"code": 2006,
		"message": "API key is invalid."
	}
}
*/

/* Example response:
{
	"location": {
		"name": "Chengdu",
		"region": "Sichuan",
		"country": "China",
		"lat": 30.6667,
		"lon": 104.0667,
		"tz_id": "Asia/Shanghai",
		"localtime_epoch": 1752645638,
		"localtime": "2025-07-16 14:00"
	},
	"current": {
		"last_updated_epoch": 1752645600,
		"last_updated": "2025-07-16 14:00",
		"temp_c": 37.3,
		"temp_f": 99.1,
		"is_day": 1,
		"condition": {
			"text": "Partly cloudy",
			"icon": "//cdn.weatherapi.com/weather/64x64/day/116.png",
			"code": 1003
		},
		"wind_mph": 2.5,
		"wind_kph": 4.0,
		"wind_degree": 128,
		"wind_dir": "SE",
		"pressure_mb": 1002.0,
		"pressure_in": 29.59,
		"precip_mm": 0.02,
		"precip_in": 0.0,
		"humidity": 51,
		"cloud": 50,
		"feelslike_c": 39.3,
		"feelslike_f": 102.7,
		"windchill_c": 34.4,
		"windchill_f": 94.0,
		"heatindex_c": 42.9,
		"heatindex_f": 109.3,
		"dewpoint_c": 25.2,
		"dewpoint_f": 77.3,
		"vis_km": 10.0,
		"vis_miles": 6.0,
		"uv": 10.4,
		"gust_mph": 2.8,
		"gust_kph": 4.6
	}
}
*/
