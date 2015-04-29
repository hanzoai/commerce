module.exports =
    payment:
      type: 'stripe'
      account:
        number: '6242424242424241'
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
      metadata:
        sleepless: true

    order:
      currency: 'usd'
      billingAddress:
        line1:      '12345 Faux Road'
        city:       'Seattle'
        state:      'Washington'
        country:    'US'
        postalCode: '55555-5555'
      shippingAddress:
        line1:      '12345 Faux Road'
        city:       'Seattle'
        state:      'Washington'
        country:    'US'
        postalCode: '55555-5555'
      items: [
        productSlug:  'doge-shirt'
        price:        100
        quantity:     2
      ]
      metadata:
        shippingNotes: 'Ship Ship to da moon.'
