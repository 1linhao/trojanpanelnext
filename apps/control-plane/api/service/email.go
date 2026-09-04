package service

import (
	"crypto/tls"
	"errors"
	"github.com/sirupsen/logrus"
	"gopkg.in/gomail.v2"
	"trojan-panel/model"
	"trojan-panel/model/constant"
	"trojan-panel/model/dto"
)

func SendEmail(sendEmailDto *dto.SendEmailDto) error {
	name := constant.SystemName
	systemVo, err := SelectSystemByName(&name)
	if err != nil {
		return err
	}
	if systemVo.EmailEnable == 0 {
		return errors.New(constant.SystemEmailError)
	}
	d := gomail.NewDialer(systemVo.EmailHost, int(systemVo.EmailPort), systemVo.EmailUsername, systemVo.EmailPassword)
	d.TLSConfig = &tls.Config{InsecureSkipVerify: true}

	go func() {
		// 插入发送记录
		toEmails := sendEmailDto.ToEmails
		for _, toEmail := range toEmails {
			// 发送消息
			m := gomail.NewMessage()
			m.SetHeaders(map[string][]string{
				"From":    {m.FormatAddress(systemVo.EmailUsername, sendEmailDto.FromEmailName)},
				"To":      {toEmail},
				"Subject": {sendEmailDto.Subject},
			})
			m.SetBody("text/html", sendEmailDto.Content)
			// 附件选项
			// m.Attach("/home/demo.jpg")

			var state int
			if err := d.DialAndSend(m); err != nil {
				logrus.Errorf("email dial and send err: %v", err)
				state = -1
			}
			state = 1
			emailRecord := model.EmailRecord{
				ToEmail: &toEmail,
				Subject: &sendEmailDto.Subject,
				Content: &sendEmailDto.Content,
				State:   &state,
			}
			if _, err = CreateEmailRecord(emailRecord); err != nil {
				logrus.Errorf("create email record err: %v", err)
				return
			}
		}
	}()
	return nil
}
