module.exports =
    payment:
      type: 'stripe'
      account:
        number: '4242424242424242'
        month:  '12'
        year:   '2016'
        cvc:    '123'
      metadata:
        paid: 'in full'

    user:
        email:     'suchfan@shirtlessinseattle.com'
        firstName: 'Sam'
        LastName:  'Ryan'
        company:   'Peabody Conservatory of Music'
        phone:     '555-555-5555'
        address:
          line1:      '12345 Faux Road'
          city:       'Seattle'
          state:      'Washington'
          country:    'United States'
          postalCode: '55555-5555'
        metadata:
          sleepless: true

    order:
      currency: 'usd'
      items: [
        productSlug:  't-shirt'
        price:        100
        quantity:     20
      ]
      metadata:
        shippingNotes: 'Ship Ship to da moon.'
