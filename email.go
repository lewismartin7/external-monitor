package main

import (
	"bytes"
	"fmt"
	"html"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"net/textproto"
	"strings"
	"time"
)

func buildEmailMessage(from string, to []string, subject, textBody, htmlBody string) ([]byte, error) {
	var alternatives bytes.Buffer
	writer := multipart.NewWriter(&alternatives)

	textHeader := textproto.MIMEHeader{}
	textHeader.Set("Content-Type", "text/plain; charset=UTF-8")
	textHeader.Set("Content-Transfer-Encoding", "quoted-printable")
	textPart, err := writer.CreatePart(textHeader)
	if err != nil {
		return nil, err
	}
	textQP := quotedprintable.NewWriter(textPart)
	if _, err = textQP.Write([]byte(textBody)); err != nil {
		return nil, err
	}
	if err = textQP.Close(); err != nil {
		return nil, err
	}

	htmlHeader := textproto.MIMEHeader{}
	htmlHeader.Set("Content-Type", "text/html; charset=UTF-8")
	htmlHeader.Set("Content-Transfer-Encoding", "quoted-printable")
	htmlPart, err := writer.CreatePart(htmlHeader)
	if err != nil {
		return nil, err
	}
	htmlQP := quotedprintable.NewWriter(htmlPart)
	if _, err = htmlQP.Write([]byte(htmlBody)); err != nil {
		return nil, err
	}
	if err = htmlQP.Close(); err != nil {
		return nil, err
	}
	if err = writer.Close(); err != nil {
		return nil, err
	}

	var message bytes.Buffer
	fmt.Fprintf(&message, "From: %s\r\n", from)
	fmt.Fprintf(&message, "To: %s\r\n", strings.Join(to, ", "))
	fmt.Fprintf(&message, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	fmt.Fprintf(&message, "Subject: %s\r\n", mime.QEncoding.Encode("UTF-8", subject))
	message.WriteString("MIME-Version: 1.0\r\n")
	fmt.Fprintf(&message, "Content-Type: multipart/alternative; boundary=%q\r\n\r\n", writer.Boundary())
	message.Write(alternatives.Bytes())
	return message.Bytes(), nil
}

func renderAlertHTML(kind, siteName, displayName string, state TargetState, message string) string {
	color, softColor, heading, badgeLabel := "#2563eb", "#dbeafe", "Monitor notification", kind
	switch kind {
	case "DOWN":
		color, softColor, heading, badgeLabel = "#dc2626", "#fee2e2", "Service unavailable", "DOWN"
	case "RECOVERED":
		color, softColor, heading, badgeLabel = "#059669", "#d1fae5", "Service recovered", "UP"
	case "CERTIFICATE WARNING":
		color, softColor, heading, badgeLabel = "#d97706", "#fef3c7", "Certificate expires soon", "EXPIRING"
	case "TEST":
		color, softColor, heading, badgeLabel = "#059669", "#d1fae5", "Email alerts are ready", "UP"
	}

	escape := html.EscapeString
	checked := "Not yet checked"
	if !state.LastChecked.IsZero() {
		checked = state.LastChecked.Format("Mon, 02 Jan 2006 15:04:05 MST")
	}
	latency := "—"
	if state.LatencyMS > 0 {
		latency = fmt.Sprintf("%d ms", state.LatencyMS)
	}
	cert := "—"
	if !state.CertExpires.IsZero() {
		cert = state.CertExpires.Format("02 Jan 2006 15:04 MST")
	}
	result := state.Detail
	if result == "" {
		result = strings.ToUpper(state.Status)
	}
	formattedMessage := strings.ReplaceAll(escape(message), "\n", "<br>")
	url := escape(state.URL)
	urlCell := url
	if strings.HasPrefix(state.URL, "https://") || strings.HasPrefix(state.URL, "http://") {
		urlCell = `<a href="` + url + `" style="color:#2563eb;text-decoration:none;word-break:break-all">` + url + `</a>`
	}

	return fmt.Sprintf(`<!doctype html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"></head>
<body style="margin:0;padding:0;background:#f3f4f6;color:#111827;font-family:Arial,Helvetica,sans-serif">
<table role="presentation" width="100%%" cellspacing="0" cellpadding="0" border="0" style="background:#f3f4f6"><tr><td align="center" style="padding:28px 12px">
<table role="presentation" width="100%%" cellspacing="0" cellpadding="0" border="0" style="max-width:600px;background:#ffffff;border-radius:14px;overflow:hidden;box-shadow:0 4px 18px rgba(17,24,39,.08)">
<tr><td style="height:7px;background:%s;font-size:0">&nbsp;</td></tr>
<tr><td style="padding:30px 32px 12px">
<div style="font-size:12px;letter-spacing:1.4px;text-transform:uppercase;color:#6b7280;font-weight:bold">%s</div>
<h1 style="margin:10px 0 8px;font-size:26px;line-height:1.25;color:#111827">%s</h1>
<span style="display:inline-block;padding:6px 11px;border-radius:999px;background:%s;color:%s;font-size:12px;font-weight:bold;letter-spacing:.5px"><span style="display:inline-block;width:9px;height:9px;margin-right:6px;border-radius:50%%;background:%s;vertical-align:0"></span>%s</span>
</td></tr>
<tr><td style="padding:12px 32px 22px"><p style="margin:0;color:#4b5563;font-size:15px;line-height:1.65">%s</p></td></tr>
<tr><td style="padding:0 32px 28px">
<table role="presentation" width="100%%" cellspacing="0" cellpadding="0" border="0" style="border:1px solid #e5e7eb;border-radius:10px;overflow:hidden">
%s%s%s%s%s%s
</table></td></tr>
<tr><td style="padding:18px 32px;background:#f9fafb;border-top:1px solid #e5e7eb;color:#6b7280;font-size:12px;line-height:1.5">%s<br>Independent monitoring from a remote Unraid server.</td></tr>
</table></td></tr></table></body></html>`,
		color, escape(displayName), escape(heading), softColor, color, color, escape(badgeLabel), formattedMessage,
		emailRow("Service", escape(state.Name)),
		emailRow("URL", urlCell),
		emailRow("Result", escape(result)),
		emailRow("Checked", escape(checked)),
		emailRow("Response time", escape(latency)),
		emailRow("Certificate expires", escape(cert)),
		escape(siteName))
}

func formatEmailAddress(displayName, address string) string {
	return (&mail.Address{Name: displayName, Address: address}).String()
}

func emailRow(label, value string) string {
	return `<tr><td style="width:38%;padding:11px 14px;border-bottom:1px solid #e5e7eb;background:#f9fafb;color:#6b7280;font-size:13px;font-weight:bold">` + label + `</td><td style="padding:11px 14px;border-bottom:1px solid #e5e7eb;color:#111827;font-size:13px">` + value + `</td></tr>`
}
