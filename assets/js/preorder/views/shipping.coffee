View = require 'mvstar/lib/view'

class ShippingView extends View
  template: '#shipping-template'
  bindings:
    Email:      ['#email        @value'
                'span.email']
    FirstName:  '#first_name    @value'
    LastName:   '#last_name     @value'
    Phone:      '#phone         @value'
    Line1:      '#address1      @value'
    Line2:      '#address2      @value'
    City:       '#city          @value'
    State:      ['#state        @value'
                '#state_province @value']
    PostalCode: '#postal_code   @value'
    Country:    '#country       @value'

  render: ->
    super
    country = @get 'Country'
    if country == "United States"
      @$el.find('intl-only').remove()
      $('.shipping').addClass('us')
      $('.shipping').removeClass('intl')
    else
      @$el.find('us-only').remove()
      $('.shipping').addClass('intl')
      $('.shipping').removeClass('us')

module.exports = ShippingView

