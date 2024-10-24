package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cloudflare/cloudflare-go"
)

const (
	ipInfoAPI1 = "https://ipinfo.io"
	ipInfoAPI2 = "https://api6.ipify.org/?format=json"	
)

var ipInfoAPIs = [...]string{
	ipInfoAPI1,
	ipInfoAPI2,
}

var (
	apiToken    = os.Getenv("APITOKEN")
	domain      = os.Getenv("DOMAIN")
	prefix      = os.Getenv("PREFIX")
	segment     = os.Getenv("SEGMENT")
	period, _   = strconv.ParseUint(os.Getenv("PERIOD"), 10, 64)
	zoneID      string
	recordID    string
	subDomain   string
	fullDomain  string
	currentZone *cloudflare.Zone
)

func main() {
	// 检查环境变量
	if apiToken == "" || domain == "" || prefix == "" {
		fmt.Println("请设置必要的环境变量: APITOKEN, DOMAIN, PREFIX, PERIOD")
		os.Exit(1)
	}
	if period == 0 {
		period = 60
	}

	subDomain = prefix
	if segment != "" {
		subDomain += "." + segment
	}
	fullDomain = subDomain + "." + domain
	fmt.Println("域名:", fullDomain)

	// 设置 Cloudflare API 密钥
	api, err := cloudflare.NewWithAPIToken(apiToken)
	if err != nil {
		fmt.Println("Cloudflare API 初始化失败:", err)
		os.Exit(1)
	}

	// 获取所有 Zones，根据顶级域名进行过滤
	zones, err := api.ListZones(context.Background(), domain)
	if err != nil {
		fmt.Println("获取 Cloudflare Zones 失败:", err)
		os.Exit(1)
	}
	// 寻找匹配的 Zone
	for _, z := range zones {
		if strings.HasSuffix(domain, z.Name) {
			currentZone = &z
			zoneID = z.ID
			fmt.Println("获取ZoneID成功:", zoneID)
			break
		}
	}
	if currentZone == nil {
		fmt.Printf("找不到与域名 %s 匹配的 Cloudflare Zone\n", domain)
		os.Exit(1)
	}
	// 定期执行更新操作
	for {
		// 获取 IPv4 地址
		currentIPv4, err := getCurrentIP(false)
		if err != nil {
			fmt.Println("获取当前外网IPv4地址失败:", err)
			continue
		}

		// 获取 IPv6 地址
		currentIPv6, err := getCurrentIP(true)
		if err != nil {
			fmt.Println("获取当前外网IPv6地址失败:", err)
			continue
		}

		comment := ""
		
		
		// 更新 A 记录（IPv4）
		err = processDNSRecord(api, zoneID, fullDomain, subDomain, currentIPv4, "A", comment)
		if err != nil {
			fmt.Println("处理 A 记录失败:", err)
		}

		// 更新 AAAA 记录（IPv6）
		err = processDNSRecord(api, zoneID, fullDomain, subDomain, currentIPv6, "AAAA", comment)
		if err != nil {
			fmt.Println("处理 AAAA 记录失败:", err)
		}

		time.Sleep(time.Duration(period) * time.Second)
	}
}


// getCurrentIP 获取当前外网地址，支持IPv4和IPv6
func getCurrentIP(ipv6 bool) (string, error) {
	for _, api := range ipInfoAPIs {

		
		resp, err := http.Get(api)
		if err != nil {
			fmt.Printf("获取外网地址失败：%s\n", err)
			continue
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Printf("读取响应体失败：%s\n", err)
			continue
		}

		var ipInfo map[string]interface{}
		err = json.Unmarshal(body, &ipInfo)
		if err != nil {
			fmt.Printf("解析 JSON 失败：%s\n", err)
			continue
		}

		
		var ip string
		ip, _ = ipInfo["ip"].(string)

		if ip != "" {
			if ipv6 && strings.Contains(ip, ":") {
				return ip, nil
				}else if!ipv6 && strings.Contains(ip, ".") {
					return ip, nil
				}
			
		}
	}
	return "", fmt.Errorf("所有 API 获取外网地址失败")
}


// getDNSRecord 获取指定的 Cloudflare DNS 记录
func getDNSRecord(api *cloudflare.API, zoneID string, name string, recordType string, comment string) ([]cloudflare.DNSRecord, error) {

	// 定义 ListDNSRecordsParams 参数
	params := cloudflare.ListDNSRecordsParams{
		Name:    name,
		Type:    recordType,// A 记录或 AAAA 记录
		Comment: comment,
	}

	// 定义 ResourceContainer
	rc := &cloudflare.ResourceContainer{Identifier: zoneID}

	// 调用 ListDNSRecords 函数
	records, _, err := api.ListDNSRecords(context.Background(), rc, params)
	if err != nil {
		return nil, err
	}
	return records, nil
}

// 新建DNS 记录
func createDNSRecord(api *cloudflare.API, zoneID string, subdomain, content string, recordType string) (cloudflare.DNSRecord, error) {
	createdRecord := cloudflare.CreateDNSRecordParams{
		Name:    subdomain,
		Content: content,
		Type:    recordType,
		Proxied: &[]bool{false}[0],
		ID:  zoneID,
	}

	rc := &cloudflare.ResourceContainer{Identifier: zoneID}

	dnsRecord, err := api.CreateDNSRecord(context.Background(), rc, createdRecord)

	return dnsRecord, err
}

// updateDNSRecord 更新指定的 Cloudflare DNS 记录，支持A和AAAA记录
func updateDNSRecord(api *cloudflare.API, recordID, zoneID, subdomain, content, recordType string) error {
	updatedRecord := cloudflare.UpdateDNSRecordParams{
		ID:      recordID,
		Name:    subdomain,
		Content: content,
		Type:    recordType, // A 或 AAAA
	}

	rc := &cloudflare.ResourceContainer{Identifier: zoneID}
	_, err := api.UpdateDNSRecord(context.Background(), rc, updatedRecord)
	return err
}
//添加 processDNSRecord 函数，用于处理 DNS 更新
func processDNSRecord(api *cloudflare.API, zoneID, fullDomain, subDomain, currentIP, recordType string,comment string) error {
	// 获取当前 DNS 记录
	dnsRecords, err := getDNSRecord(api, zoneID, fullDomain, recordType, comment)
	if err != nil {
		return fmt.Errorf("获取 %s 记录失败: %w", recordType, err)
	}
	dnsRecord := cloudflare.DNSRecord{}
	if len(dnsRecords) == 0 {
		// 创建新 DNS 记录
		dnsRecord, err = createDNSRecord(api, zoneID, subDomain, currentIP, recordType)
		if err != nil {
			return fmt.Errorf("创建 %s 记录失败: %w", recordType, err)
		}
		fmt.Printf("创建 %s 记录成功: %s => %s\n", recordType, dnsRecord.Name, currentIP)
	} else {
		dnsRecord = dnsRecords[0]
	}

	// 如果 IP 变化，更新 DNS 记录
	if currentIP != dnsRecord.Content && dnsRecord.Content != "" {
		fmt.Printf("公网%s IP变化，更新%s记录: %s => %s\n", recordType, recordType, dnsRecord.Content, currentIP)
		err := updateDNSRecord(api, dnsRecord.ID, zoneID, subDomain, currentIP, recordType)
		if err != nil {
			return fmt.Errorf("更新 %s 记录失败: %w", recordType, err)
		}
		fmt.Printf("%s 记录已更新: %s => %s\n", recordType, dnsRecord.Name, currentIP)
	} else {
		fmt.Printf("公网%s IP 与 DNS 记录一致，无需更新\n", recordType)
	}
	return nil
}
