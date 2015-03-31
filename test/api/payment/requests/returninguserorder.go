package requests

var ReturningUserOrder = `
{
  "payment": {
    "type": "stripe",
    "account": {
      "number": "4242424242424242",
      "month": "12",
      "year": "2016",
      "cvc": "123"
    }
  },
  "user": {
	"id": "%s"
  },
  "order": {
    "currency": "usd",
    "items": [ {
      "productId": "1",
      "price": 1000,
      "quantity": 3
    } ]
  }
}`
