package aws

type DescribeInstancesResponse struct {
	RequestId      string `xml:"requestId"`
	ReservationSet []struct {
		InstancesSet []struct {
			InstanceId    string `xml:"instanceId"`
			ImageId       string `xml:"imageId"`
			InstanceState struct {
				Code int    `xml:"code"`
				Name string `xml:"name"`
			} `xml:"instanceState"`
		} `xml:"instancesSet>item"`
	} `xml:"reservationSet>item"`
}
