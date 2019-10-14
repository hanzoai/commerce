package request

func CreateDispute(event, status string) string {
	return `
{
	"created": 1326853478,
	"livemode": true,
	"id": "evt_00000000000000",
	"type": "` + event + `",
	"object": "event",
	"request": null,
	"pending_webhooks": 1,
	"api_version": "2015-06-15",
	"user_id": "1",
    "object": {
	  "charge": "ch_15ZGKCCSRlllXCwPryrymFEH",
	  "amount": 2499,
	  "created": 1424675302,
	  "status": "` + status + `",
	  "livemode": false,
	  "currency": "usd",
	  "object": "dispute",
	  "reason": "general",
	  "is_charge_refundable": false,
	  "balance_transactions": [
		{
		  "id": "txn_15ZGKICSRlllXCwPg71Bwysl",
		  "object": "balance_transaction",
		  "amount": -2499,
		  "currency": "usd",
		  "net": -3999,
		  "type": "adjustment",
		  "created": 1424675302,
		  "available_on": 1424822400,
		  "status": "available",
		  "fee": 1500,
		  "fee_details": [
			{
			  "amount": 1500,
			  "currency": "usd",
			  "type": "stripe_fee",
			  "description": "Dispute fee",
			  "application": null
			}
		  ],
		  "source": "ch_15ZGKCCSRlllXCwPryrymFEH",
		  "description": "Chargeback withdrawal for ch_15ZGKCCSRlllXCwPryrymFEH",
		  "sourced_transfers": {
			"object": "list",
			"total_count": 0,
			"has_more": false,
			"url": "/v1/transfers?source_transaction=ad_15ZGKICSRlllXCwPxSSjrMvW",
			"data": [

			]
		  }
		}
	  ]
	}
}
`
}

func CreatePayment(event, orderId, paymentId, status string, refunded, captured bool) string {
	capturedStr := "false"
	if captured {
		capturedStr = "true"
	}

	refundedStr := "false"
	if refunded {
		refundedStr = "true"
	}

	return `
{
	"created": 1326853478,
	"livemode": true,
	"id": "evt_00000000000000",
	"type": "` + event + `",
	"object": "event",
	"request": null,
	"pending_webhooks": 1,
	"api_version": "2015-06-15",
	"user_id": "1",
	"data": {
		"object": {
			"id": "ch_00000000000000",
			"object": "charge",
			"created": 1436245833,
			"livemode": false,
			"paid": true,
			"status": "` + status + `",
			"amount": 1000,
			"currency": "usd",
			"refunded": ` + refundedStr + `,
			"source": {
				"id": "card_00000000000000",
				"object": "card",
				"last4": "4242",
				"brand": "Visa",
				"funding": "credit",
				"exp_month": 12,
				"exp_year": 2042,
				"country": "US",
				"name": "Fry Not Sure",
				"address_line1": null,
				"address_line2": null,
				"address_city": null,
				"address_state": null,
				"address_zip": null,
				"address_country": null,
				"cvc_check": "pass",
				"address_line1_check": null,
				"address_zip_check": null,
				"tokenization_method": null,
				"dynamic_last4": null,
				"metadata": {},
				"customer": "cus_00000000000000"
			},
			"captured": ` + capturedStr + `,
			"balance_transaction": "txn_00000000000000",
			"failure_message": null,
			"failure_code": null,
			"amount_refunded": 0,
			"customer": "cus_00000000000000",
			"invoice": null,
			"description": "Such T-shirt x2",
			"dispute": null,
			"metadata": {
				"order": "` + orderId + `",
				"payment": "` + paymentId + `"
			},
			"statement_descriptor": null,
			"fraud_details": {},
			"receipt_email": "dev@hanzo.ai",
			"receipt_number": null,
			"shipping": null,
			"destination": null,
			"application_fee": null,
			"refunds": {
				"object": "list",
				"total_count": 0,
				"has_more": false,
				"url": "/v1/charges/ch_16LoLlCSRlllXCwPVBXRc4pW/refunds",
				"data": []
			}
		}
	}
}
`
}
