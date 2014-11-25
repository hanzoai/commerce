View = require 'mvstar/lib/view'

# Swaps international shipping options if outside US.
swapInternationalOptions = do ->
  swapped = false

  (v) ->
    return if swapped

    if v == 'United States'
      @el.find('.intl-only').remove()
      $('.shipping').addClass 'us'
      $('.shipping').removeClass 'intl'
    else
      @el.find('.us-only').remove()
      $('.shipping').addClass 'intl'
      $('.shipping').removeClass 'us'
    swapped = true

class ShippingView extends View
  template: '#shipping-template'
  bindings:
    Email:     ['#email          @value'
                'span.email      @text']
    FirstName: ['#first_name     @value'
                '.first_name     @text']
    LastName:   '#last_name      @value'
    Phone:      '#phone          @value'
    Line1:      '#address1       @value'
    Line2:      '#address2       @value'
    City:       '#city           @value'
    State:     ['#state          @value'
                '#state_province @value']
    PostalCode: '#postal_code    @value'
    Country:   ['#country        @value'
                swapInternationalOptions]

  formatters:
    FirstName: (v, selector) ->
      switch selector
        when '#first_name @value'
          v
        when '.first_name @text'
          if v != ''
            ', ' + v
          else
            ''

module.exports = ShippingView
