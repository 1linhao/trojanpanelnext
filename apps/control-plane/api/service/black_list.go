package service

import (
	"fmt"
	"trojan-panel/dao"
	"trojan-panel/dao/redis"
	"trojan-panel/model/vo"
)

func DeleteBlackListByIp(ip *string) error {
	if err := dao.DeleteBlackListByIp(ip); err != nil {
		return err
	}
	_ = redis.Client.Key.RetryDel("trojan-panel:black-list:%s", *ip)
	return nil
}

func CreateBlackList(ips []string) error {
	var ipSet []string
	for _, ip := range ips {
		// 判断库中是否已经存在
		countIp, err := dao.CountBlackListByIp(&ip)
		if err != nil {
			return err
		}
		if countIp == 0 {
			ipSet = append(ipSet, ip)
		}
	}
	if err := dao.CreateBlackList(ipSet); err != nil {
		return err
	}
	kv := map[string]interface{}{}
	for _, ip := range ipSet {
		kv[fmt.Sprintf("trojan-panel:black-list:%s", ip)] = "in-black-list"
	}
	redis.Client.String.MSet(kv)
	return nil
}

func SelectBlackListPage(ip *string, pageNum *uint, pageSize *uint) (*vo.BlackListPageVo, error) {
	blackListPageVo, err := dao.SelectBlackListPage(ip, pageNum, pageSize)
	if err != nil {
		return nil, err
	}
	return blackListPageVo, nil
}
