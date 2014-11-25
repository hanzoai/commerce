package mail

import (
	"fmt"

	"github.com/gin-gonic/gin"
)

func SendMail(c *gin.Context, from_name, from_email, to_name, to_email, subject, html string) error {
	j := fmt.Sprintf(`{
    "key": "%s",
    "message": {
        "html": "%s",,
        "subject": "%s",
        "from_email": "%s",
        "from_name": "%s",
        "to": [
            {
                "email": "%s",
                "name": "%s",
                "type": "to"
            }
        ],
        "headers": {
            "Reply-To": "%s"
        },
        "important": true,
        "track_opens": true,
        "track_clicks": true,
        "auto_text": null,
        "auto_html": null,
        "inline_css": "",
        "url_strip_qs": "",
        "preserve_recipients": "",
        "view_content_link": "",
        "merge_language": "mailchimp",
        "tags": [
            "skully, preorder"
        ]
    },
    "async": false,
    "ip_pool": "Main Pool"
}`,
		apiKey,
		html,
		subject,
		from_email,
		from_name,
		to_name,
		"noreply@skullysystems.com",
	)

}
