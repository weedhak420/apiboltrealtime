package main

import (
	"net/http"
	"sync"
	"time"

	sio "github.com/googollee/go-socket.io"
)

// Location represents a location to monitor
type Location struct {
	ID          string             `json:"id"`
	Name        map[string]string  `json:"name"`
	District    string             `json:"district"`
	Type        string             `json:"type"`
	Priority    int                `json:"priority"`
	Coordinates map[string]float64 `json:"coordinates"`
}

// Vehicle represents a taxi vehicle
type Vehicle struct {
	ID             string    `json:"id"`
	Lat            float64   `json:"lat"`
	Lng            float64   `json:"lng"`
	Bearing        float64   `json:"bearing"`
	IconURL        string    `json:"icon_url"`
	CategoryName   string    `json:"category_name"`
	CategoryID     string    `json:"category_id"`
	SourceLocation string    `json:"source_location"`
	Timestamp      time.Time `json:"timestamp"`
	Distance       float64   `json:"distance,omitempty"` // Distance from center point
}

// HistoryRecord represents a vehicle history entry
type HistoryRecord struct {
	HistoryID    int64   `json:"history_id"`
	VehicleID    string  `json:"vehicle_id"`
	Lat          float64 `json:"lat"`
	Lng          float64 `json:"lng"`
	Bearing      int     `json:"bearing"`
	CategoryName string  `json:"category_name"`
	Timestamp    string  `json:"timestamp"`
	CreatedAt    string  `json:"created_at"`
}

// VehicleHistory represents a record from vehicle_history for the new API
type VehicleHistory struct {
	ID           string    `json:"id"`
	Lat          float64   `json:"lat"`
	Lng          float64   `json:"lng"`
	Bearing      float64   `json:"bearing"`
	Timestamp    time.Time `json:"timestamp"`
	CategoryName string    `json:"category_name"`
}

// Hotspot represents a heatmap hotspot
type Hotspot struct {
	GridLat  float64 `json:"grid_lat"`
	GridLng  float64 `json:"grid_lng"`
	Vehicles int     `json:"vehicles"`
}

// TrendPoint represents a trend data point
type TrendPoint struct {
	Time     string  `json:"time"`
	Vehicles int     `json:"vehicles"`
	Smoothed float64 `json:"smoothed,omitempty"`
}

// APIResponse represents the Bolt API response
type APIResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Vehicles struct {
			Taxi  map[string][]VehicleData `json:"taxi"`
			Icons struct {
				Taxi map[string]IconData `json:"taxi"`
			} `json:"icons"`
			CategoryDetails struct {
				Taxi map[string]CategoryData `json:"taxi"`
			} `json:"category_details"`
		} `json:"vehicles"`
	} `json:"data"`
}

type VehicleData struct {
	ID      string  `json:"id"`
	Lat     float64 `json:"lat"`
	Lng     float64 `json:"lng"`
	Bearing float64 `json:"bearing"`
	IconID  string  `json:"icon_id"`
}

type IconData struct {
	IconURL string `json:"icon_url"`
}

type CategoryData struct {
	Name string `json:"name"`
}

// LocationResult represents the result of fetching from one location
type LocationResult struct {
	Location string
	Success  bool
	Vehicles []Vehicle
	Error    string
}

// AllLocations contains all monitoring locations for Chiang Mai
var AllLocations = []Location{
	// ===========================================
	// 🏙️ MAIN CITY AREAS (พื้นที่หลักในเมือง)
	// ===========================================
	{
		ID:          "city_center",
		Name:        map[string]string{"th": "ศูนย์กลางเมือง", "en": "City Center"},
		District:    "Mueang Chiang Mai",
		Type:        "urban",
		Priority:    1,
		Coordinates: map[string]float64{"lat": 18.7883, "lng": 98.9853},
	},
	{
		ID:          "old_city",
		Name:        map[string]string{"th": "เมืองเก่า", "en": "Old City"},
		District:    "Mueang Chiang Mai",
		Type:        "historic",
		Priority:    1,
		Coordinates: map[string]float64{"lat": 18.7912, "lng": 98.9853},
	},
	{
		ID:          "tha_phae_gate",
		Name:        map[string]string{"th": "ประตูท่าแพ", "en": "Tha Phae Gate"},
		District:    "Mueang Chiang Mai",
		Type:        "historic",
		Priority:    1,
		Coordinates: map[string]float64{"lat": 18.7868, "lng": 98.9931},
	},
	{
		ID:          "nimman",
		Name:        map[string]string{"th": "นิมมาน", "en": "Nimman"},
		District:    "Mueang Chiang Mai",
		Type:        "lifestyle",
		Priority:    1,
		Coordinates: map[string]float64{"lat": 18.8002, "lng": 98.9679},
	},
	{
		ID:          "cmu_area",
		Name:        map[string]string{"th": "มช.", "en": "CMU Area"},
		District:    "Mueang Chiang Mai",
		Type:        "education",
		Priority:    2,
		Coordinates: map[string]float64{"lat": 18.8063, "lng": 98.9511},
	},
	{
		ID:          "maya_mall",
		Name:        map[string]string{"th": "มายา", "en": "Maya Mall"},
		District:    "Mueang Chiang Mai",
		Type:        "shopping",
		Priority:    2,
		Coordinates: map[string]float64{"lat": 18.8025, "lng": 98.9667},
	},
	{
		ID:          "airport",
		Name:        map[string]string{"th": "สนามบิน", "en": "Airport"},
		District:    "Mueang Chiang Mai",
		Type:        "transport",
		Priority:    1,
		Coordinates: map[string]float64{"lat": 18.7667, "lng": 98.9625},
	},
	{
		ID:          "san_kamphaeng",
		Name:        map[string]string{"th": "สันกำแพง", "en": "San Kamphaeng"},
		District:    "San Kamphaeng",
		Type:        "suburban",
		Priority:    3,
		Coordinates: map[string]float64{"lat": 18.75, "lng": 99.1167},
	},
	{
		ID:          "hang_dong",
		Name:        map[string]string{"th": "หางดง", "en": "Hang Dong"},
		District:    "Hang Dong",
		Type:        "suburban",
		Priority:    3,
		Coordinates: map[string]float64{"lat": 18.6833, "lng": 98.9167},
	},
	{
		ID:          "doi_saket",
		Name:        map[string]string{"th": "ดอยสะเก็ด", "en": "Doi Saket"},
		District:    "Doi Saket",
		Type:        "suburban",
		Priority:    3,
		Coordinates: map[string]float64{"lat": 18.9167, "lng": 99.1667},
	},
	{
		ID:          "mae_rim",
		Name:        map[string]string{"th": "แม่ริม", "en": "Mae Rim"},
		District:    "Mae Rim",
		Type:        "suburban",
		Priority:    2,
		Coordinates: map[string]float64{"lat": 18.9167, "lng": 98.8833},
	},
	{
		ID:          "doi_suthep",
		Name:        map[string]string{"th": "ดอยสุเทพ", "en": "Doi Suthep"},
		District:    "Mueang Chiang Mai",
		Type:        "nature",
		Priority:    2,
		Coordinates: map[string]float64{"lat": 18.8047, "lng": 98.9217},
	},
	{
		ID:          "san_sai",
		Name:        map[string]string{"th": "สันทราย", "en": "San Sai"},
		District:    "San Sai",
		Type:        "suburban",
		Priority:    3,
		Coordinates: map[string]float64{"lat": 18.8667, "lng": 99.0333},
	},
	{
		ID:          "saraphi",
		Name:        map[string]string{"th": "สารภี", "en": "Saraphi"},
		District:    "Saraphi",
		Type:        "suburban",
		Priority:    3,
		Coordinates: map[string]float64{"lat": 18.7167, "lng": 99.0167},
	},

	// ===========================================
	// 🕌 TEMPLES (วัดสำคัญ)
	// ===========================================
	{
		ID:          "wat_chedi_luang",
		Name:        map[string]string{"th": "วัดเจดีย์หลวง", "en": "Wat Chedi Luang"},
		District:    "Mueang Chiang Mai",
		Type:        "temple",
		Priority:    2,
		Coordinates: map[string]float64{"lat": 18.7867, "lng": 98.9878},
	},
	{
		ID:          "wat_phra_singh",
		Name:        map[string]string{"th": "วัดพระสิงห์", "en": "Wat Phra Singh"},
		District:    "Mueang Chiang Mai",
		Type:        "temple",
		Priority:    2,
		Coordinates: map[string]float64{"lat": 18.7869, "lng": 98.9831},
	},
	{
		ID:          "wat_phra_that_doi_suthep",
		Name:        map[string]string{"th": "วัดพระธาตุดอยสุเทพ", "en": "Wat Phra That Doi Suthep"},
		District:    "Mueang Chiang Mai",
		Type:        "temple",
		Priority:    1,
		Coordinates: map[string]float64{"lat": 18.8047, "lng": 98.9217},
	},
	{
		ID:          "wat_umong",
		Name:        map[string]string{"th": "วัดอุโมงค์", "en": "Wat Umong"},
		District:    "Mueang Chiang Mai",
		Type:        "temple",
		Priority:    2,
		Coordinates: map[string]float64{"lat": 18.7650, "lng": 98.9300},
	},
	{
		ID:          "wat_doi_kham",
		Name:        map[string]string{"th": "วัดพระธาตุดอยคำ", "en": "Wat Doi Kham"},
		District:    "Mueang Chiang Mai",
		Type:        "temple",
		Priority:    3,
		Coordinates: map[string]float64{"lat": 18.7331, "lng": 98.9094},
	},
	{
		ID:          "wat_suan_dok",
		Name:        map[string]string{"th": "วัดสวนดอก", "en": "Wat Suan Dok"},
		District:    "Mueang Chiang Mai",
		Type:        "temple",
		Priority:    2,
		Coordinates: map[string]float64{"lat": 18.7881, "lng": 98.9714},
	},

	// ===========================================
	// 🛍️ SHOPPING & MARKETS (ช้อปปิ้งและตลาด)
	// ===========================================
	{
		ID:          "warorot_market",
		Name:        map[string]string{"th": "ตลาดวโรรส", "en": "Warorot Market"},
		District:    "Mueang Chiang Mai",
		Type:        "market",
		Priority:    2,
		Coordinates: map[string]float64{"lat": 18.7903, "lng": 98.9931},
	},
	{
		ID:          "sunday_walking_street",
		Name:        map[string]string{"th": "ถนนคนเดินวันอาทิตย์", "en": "Sunday Walking Street"},
		District:    "Mueang Chiang Mai",
		Type:        "market",
		Priority:    1,
		Coordinates: map[string]float64{"lat": 18.7906, "lng": 98.9867},
	},
	{
		ID:          "saturday_walking_street",
		Name:        map[string]string{"th": "ถนนคนเดินวันเสาร์", "en": "Saturday Walking Street"},
		District:    "Mueang Chiang Mai",
		Type:        "market",
		Priority:    2,
		Coordinates: map[string]float64{"lat": 18.7850, "lng": 98.9950},
	},
	{
		ID:          "central_festival",
		Name:        map[string]string{"th": "เซ็นทรัลเฟสติวัล", "en": "Central Festival"},
		District:    "Mueang Chiang Mai",
		Type:        "shopping",
		Priority:    2,
		Coordinates: map[string]float64{"lat": 18.8028, "lng": 98.9531},
	},
	{
		ID:          "one_nimman",
		Name:        map[string]string{"th": "วัน นิมมาน", "en": "One Nimman"},
		District:    "Mueang Chiang Mai",
		Type:        "shopping",
		Priority:    2,
		Coordinates: map[string]float64{"lat": 18.7969, "lng": 98.9667},
	},
	{
		ID:          "kad_suan_kaew",
		Name:        map[string]string{"th": "กาดสวนแก้ว", "en": "Kad Suan Kaew"},
		District:    "Mueang Chiang Mai",
		Type:        "shopping",
		Priority:    2,
		Coordinates: map[string]float64{"lat": 18.8008, "lng": 98.9689},
	},
	{
		ID:          "think_park",
		Name:        map[string]string{"th": "ติ๊งค์พาร์ค", "en": "Think Park"},
		District:    "Mueang Chiang Mai",
		Type:        "lifestyle",
		Priority:    2,
		Coordinates: map[string]float64{"lat": 18.8019, "lng": 98.9644},
	},

	// ===========================================
	// 🌳 NATURE & ATTRACTIONS (ธรรมชาติและสถานที่ท่องเที่ยว)
	// ===========================================
	{
		ID:          "chiang_mai_zoo",
		Name:        map[string]string{"th": "สวนสัตว์เชียงใหม่", "en": "Chiang Mai Zoo"},
		District:    "Mueang Chiang Mai",
		Type:        "nature",
		Priority:    2,
		Coordinates: map[string]float64{"lat": 18.8028, "lng": 98.9217},
	},
	{
		ID:          "doi_inthanon",
		Name:        map[string]string{"th": "ดอยอินทนนท์", "en": "Doi Inthanon"},
		District:    "Chom Thong",
		Type:        "nature",
		Priority:    3,
		Coordinates: map[string]float64{"lat": 18.5881, "lng": 98.4872},
	},
	{
		ID:          "grand_canyon_chiangmai",
		Name:        map[string]string{"th": "แกรนด์แคนยอน", "en": "Grand Canyon Chiang Mai"},
		District:    "Hang Dong",
		Type:        "nature",
		Priority:    3,
		Coordinates: map[string]float64{"lat": 18.7231, "lng": 99.0531},
	},
	{
		ID:          "royal_park_rajapruek",
		Name:        map[string]string{"th": "อุทยานหลวงราชพฤกษ์", "en": "Royal Park Rajapruek"},
		District:    "Mae Rim",
		Type:        "nature",
		Priority:    2,
		Coordinates: map[string]float64{"lat": 18.8156, "lng": 98.8800},
	},
	{
		ID:          "queen_sirikit_botanic_garden",
		Name:        map[string]string{"th": "สวนพฤกษศาสตร์สมเด็จพระนางเจ้าสิริกิติ์", "en": "Queen Sirikit Botanic Garden"},
		District:    "Mae Rim",
		Type:        "nature",
		Priority:    3,
		Coordinates: map[string]float64{"lat": 18.8917, "lng": 98.8578},
	},
	{
		ID:          "mon_jam",
		Name:        map[string]string{"th": "มอญแจ่ม", "en": "Mon Jam"},
		District:    "Mae Rim",
		Type:        "nature",
		Priority:    3,
		Coordinates: map[string]float64{"lat": 18.9500, "lng": 98.8333},
	},
	{
		ID:          "bua_thong_waterfall",
		Name:        map[string]string{"th": "น้ำตกบัวตอง", "en": "Bua Thong Waterfall"},
		District:    "Mae Taeng",
		Type:        "nature",
		Priority:    3,
		Coordinates: map[string]float64{"lat": 19.0667, "lng": 98.8833},
	},

	// ===========================================
	// 🚉 TRANSPORT HUBS (ศูนย์คมนาคม)
	// ===========================================
	{
		ID:          "train_station",
		Name:        map[string]string{"th": "สถานีรถไฟเชียงใหม่", "en": "Chiang Mai Railway Station"},
		District:    "Mueang Chiang Mai",
		Type:        "transport",
		Priority:    2,
		Coordinates: map[string]float64{"lat": 18.7972, "lng": 99.0100},
	},
	{
		ID:          "arcade_bus_station",
		Name:        map[string]string{"th": "สถานีขนส่งอาเขต", "en": "Arcade Bus Station"},
		District:    "Mueang Chiang Mai",
		Type:        "transport",
		Priority:    2,
		Coordinates: map[string]float64{"lat": 18.7411, "lng": 98.9717},
	},
	{
		ID:          "chang_phuak_bus_station",
		Name:        map[string]string{"th": "สถานีขนส่งช้างเผือก", "en": "Chang Phuak Bus Station"},
		District:    "Mueang Chiang Mai",
		Type:        "transport",
		Priority:    2,
		Coordinates: map[string]float64{"lat": 18.8047, "lng": 98.9856},
	},

	// ===========================================
	// 🏨 MAJOR AREAS (พื้นที่สำคัญ)
	// ===========================================
	{
		ID:          "chang_klan",
		Name:        map[string]string{"th": "ช้างคลาน", "en": "Chang Klan"},
		District:    "Mueang Chiang Mai",
		Type:        "urban",
		Priority:    1,
		Coordinates: map[string]float64{"lat": 18.7856, "lng": 98.9997},
	},
	{
		ID:          "chang_phuak",
		Name:        map[string]string{"th": "ช้างเผือก", "en": "Chang Phuak"},
		District:    "Mueang Chiang Mai",
		Type:        "urban",
		Priority:    2,
		Coordinates: map[string]float64{"lat": 18.8033, "lng": 98.9847},
	},
	{
		ID:          "santitham",
		Name:        map[string]string{"th": "สันติธรรม", "en": "Santitham"},
		District:    "Mueang Chiang Mai",
		Type:        "urban",
		Priority:    2,
		Coordinates: map[string]float64{"lat": 18.8000, "lng": 98.9800},
	},
	{
		ID:          "nong_hoi",
		Name:        map[string]string{"th": "หนองหอย", "en": "Nong Hoi"},
		District:    "Mueang Chiang Mai",
		Type:        "urban",
		Priority:    2,
		Coordinates: map[string]float64{"lat": 18.7553, "lng": 99.0133},
	},

	// ===========================================
	// 🏘️ OUTER DISTRICTS (อำเภอนอก)
	// ===========================================
	{
		ID:          "mae_taeng_town",
		Name:        map[string]string{"th": "ตัวเมืองแม่แตง", "en": "Mae Taeng Town"},
		District:    "Mae Taeng",
		Type:        "suburban",
		Priority:    3,
		Coordinates: map[string]float64{"lat": 19.1167, "lng": 98.9500},
	},
	{
		ID:          "chiang_dao_town",
		Name:        map[string]string{"th": "ตัวเมืองเชียงดาว", "en": "Chiang Dao Town"},
		District:    "Chiang Dao",
		Type:        "suburban",
		Priority:    3,
		Coordinates: map[string]float64{"lat": 19.3667, "lng": 98.9667},
	},
	{
		ID:          "chiang_dao_cave",
		Name:        map[string]string{"th": "ถ้ำเชียงดาว", "en": "Chiang Dao Cave"},
		District:    "Chiang Dao",
		Type:        "nature",
		Priority:    3,
		Coordinates: map[string]float64{"lat": 19.4053, "lng": 98.9625},
	},
	{
		ID:          "fang_town",
		Name:        map[string]string{"th": "ตัวเมืองฝาง", "en": "Fang Town"},
		District:    "Fang",
		Type:        "suburban",
		Priority:    3,
		Coordinates: map[string]float64{"lat": 19.9167, "lng": 99.2167},
	},
	{
		ID:          "mae_ai_town",
		Name:        map[string]string{"th": "ตัวเมืองแม่อาย", "en": "Mae Ai Town"},
		District:    "Mae Ai",
		Type:        "suburban",
		Priority:    3,
		Coordinates: map[string]float64{"lat": 20.0667, "lng": 99.2833},
	},
	{
		ID:          "samoeng_town",
		Name:        map[string]string{"th": "ตัวเมืองสะเมิง", "en": "Samoeng Town"},
		District:    "Samoeng",
		Type:        "suburban",
		Priority:    3,
		Coordinates: map[string]float64{"lat": 18.7667, "lng": 98.7167},
	},
	{
		ID:          "mae_wang_town",
		Name:        map[string]string{"th": "ตัวเมืองแม่วาง", "en": "Mae Wang Town"},
		District:    "Mae Wang",
		Type:        "suburban",
		Priority:    3,
		Coordinates: map[string]float64{"lat": 18.6500, "lng": 98.6333},
	},
	{
		ID:          "mae_chaem_town",
		Name:        map[string]string{"th": "ตัวเมืองแม่แจ่ม", "en": "Mae Chaem Town"},
		District:    "Mae Chaem",
		Type:        "suburban",
		Priority:    3,
		Coordinates: map[string]float64{"lat": 18.5500, "lng": 98.4167},
	},
	{
		ID:          "chom_thong_town",
		Name:        map[string]string{"th": "ตัวเมืองจอมทอง", "en": "Chom Thong Town"},
		District:    "Chom Thong",
		Type:        "suburban",
		Priority:    3,
		Coordinates: map[string]float64{"lat": 18.4167, "lng": 98.6667},
	},
	{
		ID:          "doi_tao_town",
		Name:        map[string]string{"th": "ตัวเมืองดอยเต่า", "en": "Doi Tao Town"},
		District:    "Doi Tao",
		Type:        "suburban",
		Priority:    3,
		Coordinates: map[string]float64{"lat": 17.9167, "lng": 98.6000},
	},
	{
		ID:          "hot_town",
		Name:        map[string]string{"th": "ตัวเมืองฮอด", "en": "Hot Town"},
		District:    "Hot",
		Type:        "suburban",
		Priority:    3,
		Coordinates: map[string]float64{"lat": 17.9500, "lng": 98.4167},
	},
	{
		ID:          "doi_lo_town",
		Name:        map[string]string{"th": "ตัวเมืองดอยหล่อ", "en": "Doi Lo Town"},
		District:    "Doi Lo",
		Type:        "suburban",
		Priority:    3,
		Coordinates: map[string]float64{"lat": 18.0667, "lng": 98.2667},
	},
	{
		ID:          "omkoi_town",
		Name:        map[string]string{"th": "ตัวเมืองอมก๋อย", "en": "Omkoi Town"},
		District:    "Omkoi",
		Type:        "suburban",
		Priority:    3,
		Coordinates: map[string]float64{"lat": 17.8000, "lng": 98.4333},
	},
	{
		ID:          "mae_on_town",
		Name:        map[string]string{"th": "ตัวเมืองแม่ออน", "en": "Mae On Town"},
		District:    "Mae On",
		Type:        "suburban",
		Priority:    3,
		Coordinates: map[string]float64{"lat": 18.8333, "lng": 99.2500},
	},
	{
		ID:          "san_pa_tong_town",
		Name:        map[string]string{"th": "ตัวเมืองสันป่าตอง", "en": "San Pa Tong Town"},
		District:    "San Pa Tong",
		Type:        "suburban",
		Priority:    3,
		Coordinates: map[string]float64{"lat": 18.6333, "lng": 98.8667},
	},

	// ===========================================
	// 🌙 NIGHTLIFE (ไนท์ไลฟ์)
	// ===========================================
	{
		ID:          "zoe_in_yellow",
		Name:        map[string]string{"th": "โซอิน เยลโล", "en": "Zoe in Yellow"},
		District:    "Mueang Chiang Mai",
		Type:        "nightlife",
		Priority:    2,
		Coordinates: map[string]float64{"lat": 18.7914, "lng": 98.9900},
	},
	{
		ID:          "nimman_nightlife",
		Name:        map[string]string{"th": "ย่านนิมมานกลางคืน", "en": "Nimman Nightlife Area"},
		District:    "Mueang Chiang Mai",
		Type:        "nightlife",
		Priority:    2,
		Coordinates: map[string]float64{"lat": 18.8000, "lng": 98.9670},
	},
	{
		ID:          "loi_kroh",
		Name:        map[string]string{"th": "ลอยเคราะห์", "en": "Loi Kroh"},
		District:    "Mueang Chiang Mai",
		Type:        "nightlife",
		Priority:    2,
		Coordinates: map[string]float64{"lat": 18.7875, "lng": 98.9967},
	},

	// ===========================================
	// 🏫 UNIVERSITIES & EDUCATION (มหาวิทยาลัยและสถาบันการศึกษา)
	// ===========================================
	{
		ID:          "cmu_engineering",
		Name:        map[string]string{"th": "คณะวิศวกรรมศาสตร์ มช.", "en": "CMU Engineering"},
		District:    "Mueang Chiang Mai",
		Type:        "education",
		Priority:    2,
		Coordinates: map[string]float64{"lat": 18.8089, "lng": 98.9533},
	},
	{
		ID:          "cmu_medicine",
		Name:        map[string]string{"th": "คณะแพทยศาสตร์ มช.", "en": "CMU Medicine"},
		District:    "Mueang Chiang Mai",
		Type:        "education",
		Priority:    2,
		Coordinates: map[string]float64{"lat": 18.7933, "lng": 98.9542},
	},
	{
		ID:          "rajamangala_chiang_mai",
		Name:        map[string]string{"th": "ราชมงคลล้านนา", "en": "RMUTL Chiang Mai"},
		District:    "Mueang Chiang Mai",
		Type:        "education",
		Priority:    3,
		Coordinates: map[string]float64{"lat": 18.8167, "lng": 99.0167},
	},
	{
		ID:          "payap_university",
		Name:        map[string]string{"th": "มหาวิทยาลัยพายัพ", "en": "Payap University"},
		District:    "Mueang Chiang Mai",
		Type:        "education",
		Priority:    3,
		Coordinates: map[string]float64{"lat": 18.8042, "lng": 98.9492},
	},

	// ===========================================
	// 🏥 HOSPITALS (โรงพยาบาล)
	// ===========================================
	{
		ID:          "maharaj_hospital",
		Name:        map[string]string{"th": "โรงพยาบาลมหาราช", "en": "Maharaj Hospital"},
		District:    "Mueang Chiang Mai",
		Type:        "healthcare",
		Priority:    2,
		Coordinates: map[string]float64{"lat": 18.7950, "lng": 98.9558},
	},
	{
		ID:          "mccormick_hospital",
		Name:        map[string]string{"th": "โรงพยาบาลแมคคอร์มิค", "en": "McCormick Hospital"},
		District:    "Mueang Chiang Mai",
		Type:        "healthcare",
		Priority:    2,
		Coordinates: map[string]float64{"lat": 18.7919, "lng": 99.0106},
	},
	{
		ID:          "chiang_mai_ram_hospital",
		Name:        map[string]string{"th": "โรงพยาบาลเชียงใหม่ราม", "en": "Chiang Mai Ram Hospital"},
		District:    "Mueang Chiang Mai",
		Type:        "healthcare",
		Priority:    2,
		Coordinates: map[string]float64{"lat": 18.7947, "lng": 98.9956},
	},
	{
		ID:          "lanna_hospital",
		Name:        map[string]string{"th": "โรงพยาบาลล้านนา", "en": "Lanna Hospital"},
		District:    "Mueang Chiang Mai",
		Type:        "healthcare",
		Priority:    2,
		Coordinates: map[string]float64{"lat": 18.8097, "lng": 98.9681},
	},

	// ===========================================
	// 🏨 MAJOR HOTELS & RESORTS (โรงแรมและรีสอร์ท)
	// ===========================================
	{
		ID:          "shangri_la",
		Name:        map[string]string{"th": "แชงกรีล่า", "en": "Shangri-La Hotel"},
		District:    "Mueang Chiang Mai",
		Type:        "hotel",
		Priority:    2,
		Coordinates: map[string]float64{"lat": 18.7889, "lng": 99.0069},
	},
	{
		ID:          "le_meridien",
		Name:        map[string]string{"th": "เลอ เมอริเดียน", "en": "Le Meridien"},
		District:    "Mueang Chiang Mai",
		Type:        "hotel",
		Priority:    2,
		Coordinates: map[string]float64{"lat": 18.7869, "lng": 99.0056},
	},
	{
		ID:          "137_pillars_house",
		Name:        map[string]string{"th": "137 พิลลาร์ส เฮาส์", "en": "137 Pillars House"},
		District:    "Mueang Chiang Mai",
		Type:        "hotel",
		Priority:    2,
		Coordinates: map[string]float64{"lat": 18.7903, "lng": 99.0097},
	},
	{
		ID:          "four_seasons",
		Name:        map[string]string{"th": "โฟร์ซีซั่น", "en": "Four Seasons Resort"},
		District:    "Mae Rim",
		Type:        "hotel",
		Priority:    3,
		Coordinates: map[string]float64{"lat": 18.8658, "lng": 98.8514},
	},

	// ===========================================
	// 🍜 FOOD AREAS (ย่านอาหาร)
	// ===========================================
	{
		ID:          "somphet_market",
		Name:        map[string]string{"th": "ตลาดสมเพชร", "en": "Somphet Market"},
		District:    "Mueang Chiang Mai",
		Type:        "market",
		Priority:    2,
		Coordinates: map[string]float64{"lat": 18.7947, "lng": 98.9817},
	},
	{
		ID:          "jingjai_market",
		Name:        map[string]string{"th": "ตลาดจิ้งจ้าย", "en": "JingJai Market"},
		District:    "Mueang Chiang Mai",
		Type:        "market",
		Priority:    2,
		Coordinates: map[string]float64{"lat": 18.8028, "lng": 99.0194},
	},
	{
		ID:          "ton_payom_market",
		Name:        map[string]string{"th": "ตลาดต้นพยอม", "en": "Ton Payom Market"},
		District:    "Mueang Chiang Mai",
		Type:        "market",
		Priority:    2,
		Coordinates: map[string]float64{"lat": 18.8158, "lng": 98.9711},
	},
	{
		ID:          "ploen_ruedee_market",
		Name:        map[string]string{"th": "ตลาดเปิ้ลเหรอดี", "en": "Ploen Ruedee Market"},
		District:    "Mueang Chiang Mai",
		Type:        "market",
		Priority:    2,
		Coordinates: map[string]float64{"lat": 18.7828, "lng": 98.9803},
	},

	// ===========================================
	// 🎪 ENTERTAINMENT & CULTURE (บันเทิงและวัฒนธรรม)
	// ===========================================
	{
		ID:          "night_bazaar",
		Name:        map[string]string{"th": "ไนท์บาซาร์", "en": "Night Bazaar"},
		District:    "Mueang Chiang Mai",
		Type:        "market",
		Priority:    1,
		Coordinates: map[string]float64{"lat": 18.7881, "lng": 99.0014},
	},
	{
		ID:          "kalare_night_bazaar",
		Name:        map[string]string{"th": "กาละแม ไนท์บาซาร์", "en": "Kalare Night Bazaar"},
		District:    "Mueang Chiang Mai",
		Type:        "market",
		Priority:    2,
		Coordinates: map[string]float64{"lat": 18.7886, "lng": 99.0000},
	},
	{
		ID:          "kad_manee_market",
		Name:        map[string]string{"th": "กาดมณี", "en": "Kad Manee Market"},
		District:    "Mueang Chiang Mai",
		Type:        "market",
		Priority:    2,
		Coordinates: map[string]float64{"lat": 18.7836, "lng": 98.9667},
	},
	{
		ID:          "promenada",
		Name:        map[string]string{"th": "พรอมานาด้า", "en": "Promenada Resort Mall"},
		District:    "Mueang Chiang Mai",
		Type:        "shopping",
		Priority:    2,
		Coordinates: map[string]float64{"lat": 18.8306, "lng": 98.9614},
	},
	{
		ID:          "central_airport_plaza",
		Name:        map[string]string{"th": "เซ็นทรัลแอร์พอร์ตพลาซ่า", "en": "Central Airport Plaza"},
		District:    "Mueang Chiang Mai",
		Type:        "shopping",
		Priority:    2,
		Coordinates: map[string]float64{"lat": 18.7694, "lng": 98.9672},
	},
}

// Global variables for caching and rate limiting
var (
	vehicleCache   map[string]Vehicle // Cache for deduplication
	cacheMu        sync.RWMutex
	rateLimiter    map[string]time.Time
	rateLimitMutex sync.RWMutex
	jsonPool       sync.Pool
	responsePool   sync.Pool
	httpClient     *http.Client
	socketServer   *sio.Server
)
