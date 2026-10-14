package main

import (
	"fmt"

	"github.com/mmcloughlin/geohash"
	pb "github.com/sikozonpc/ride-sharing/shared/proto/driver"
)

// Predefined routes for drivers
// Get this coordinates from Google Maps for example and build a route
var predefinedRoutes = [][][]float64{
	{
		// 长安街沿线（东西向）
		{39.9068, 116.3746}, // 西单
		{39.9087, 116.3975}, // 天安门
		{39.9147, 116.4109}, // 王府井
		{39.9085, 116.4350}, // 建国门
	},
	{
		// 西二环 / 金融街
		{39.9069, 116.3560}, // 复兴门
		{39.9144, 116.3548}, // 金融街
		{39.9230, 116.3560}, // 阜成门
		{39.9180, 116.3450}, // 月坛
	},
	{
		// 中关村 / 五道口（西北高校区）
		{39.9835, 116.3078}, // 中关村
		{39.9760, 116.3180}, // 海淀黄庄
		{39.9921, 116.3376}, // 五道口
		{39.9880, 116.3510}, // 学院路
	},
	{
		// 国贸 CBD（东三环）
		{39.9087, 116.4601}, // 国贸
		{39.9080, 116.4760}, // 大望路
		{39.8920, 116.4600}, // 双井
		{39.8830, 116.4550}, // 劲松
	},
}

type Driver struct {
	DriverId string
	Route    [][]float64
	Index    int // Current position in the route
}

type Service struct {
	drivers []*Driver
}

func NewService() *Service {
	var drivers []*Driver
	for i, route := range predefinedRoutes {
		drivers = append(drivers, &Driver{
			DriverId: fmt.Sprintf("driver-%d", i),
			Route:    route,
			Index:    0,
		})
	}

	return &Service{
		drivers,
	}
}

func (s *Service) FindNearbyDrivers() []*pb.Driver {
	var drivers []*pb.Driver

	for _, driver := range s.drivers {
		// Get current position and move to the next waypoint
		lat := driver.Route[driver.Index][0]
		lng := driver.Route[driver.Index][1]
		driver.Index = (driver.Index + 1) % len(driver.Route) // Loop back at end of route

		// Update driver protobuf object
		pbDriver := &pb.Driver{
			DriverId: driver.DriverId,
			Location: &pb.Location{
				Latitude:  float32(lat),
				Longitude: float32(lng),
			},
			Geohash: geohash.Encode(lat, lng),
		}
		drivers = append(drivers, pbDriver)
	}

	return drivers
}
