package test

var validOrder = `
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
    "email": "suchfan@shirtlessinseattle.com",
    "firstName": "Sam",
    "LastName": "Ryan",
    "address": {
      "line1": "12345 Faux Road",
      "city": "Seattle",
      "state": "Washington",
      "country": "United States",
      "postalCode": "55555-5555"
    }
  },
  "order": {
    "currency": "usd",
    "items": [ {
      "productId": "1",
      "price": 100,
      "quantity": 20
    } ]
  }
}`
