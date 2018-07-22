package sendgrid

import (
	"context"
	"time"

	"google.golang.org/appengine"
	"google.golang.org/appengine/urlfetch"

	"github.com/sendgrid/rest"
	"github.com/sendgrid/sendgrid-go"

	"hanzo.io/types/integration"
)

// func main() {
// 	from := mail.NewEmail("Example User", "test@example.com")
// 	subject := "Sending with SendGrid is Fun"
// 	to := mail.NewEmail("Example User", "test@example.com")
// 	plainTextContent := "and easy to do anywhere, even with Go"
// 	htmlContent := "<strong>and easy to do anywhere, even with Go</strong>"
// 	message := mail.NewSingleEmail(from, subject, to, plainTextContent, htmlContent)
// 	client := sendgrid.NewSendClient(os.Getenv("SENDGRID_API_KEY"))
// 	response, err := client.Send(message)
// 	if err != nil {
// 		log.Println(err)
// 	} else {
// 		fmt.Println(response.StatusCode)
// 		fmt.Println(response.Body)
// 		fmt.Println(response.Headers)
// 	}
// }

func New(ctx context.Context, settings integration.SendGrid) *Client {
	// Set deadline
	ctx, _ = context.WithTimeout(ctx, time.Second*55)

	// Set HTTP Client for App engine
	httpClient := urlfetch.Client(ctx)

	httpClient.Transport = &urlfetch.Transport{
		Context: ctx,
		AllowInvalidServerCertificate: appengine.IsDevAppServer(),
	}
	rest.DefaultClient = &rest.Client{HTTPClient: httpClient}
	client := sendgrid.NewSendClient(settings.APIKey)

	return &Client{ctx, client}
}
