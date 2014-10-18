package aws

type CreateSecurityGroupResponse struct {
        RequestId string `xml:"requestId"`
        Return    bool   `xml:"return"`
        GroupId   string `xml:"groupId"`
}
