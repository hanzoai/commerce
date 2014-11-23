View = require 'mvstar/lib/view'
{getVariant} = require '../../utils/products'

cart = app.get 'cart'

class ProductView extends View
  el: '.sqs-add-to-cart-button'

  events:
    click: -> @addToCart()

  addToCart: ->
    unless (variant = @getVariant())?
      return

    quantity = parseInt $("#quantity").val(), 10

    inner = $('.sqs-add-to-cart-button-inner')
    inner.html ''
    inner.append '<div class="yui3-widget sqs-spin light"></div>'
    inner.append '<div class="status-text">Adding...</div>'

    cart.add
      sku:      variant.SKU
      color:    variant.Color
      img:      currentProduct.Images[0].Url
      name:     currentProduct.Title
      price:    parseInt(variant.Price, 10) * 0.0001
      quantity: quantity
      size:     variant.Size
      slug:     currentProduct.Slug

    setTimeout ->
      $('.status-text').text('Added!').fadeOut 500, ->
        inner.html 'Add to Cart'
    , 500

    setTimeout ->
      # Flash cart hover
      $('.sqs-pill-shopping-cart-content').animate opacity: 0.85, 400, ->
        # Update cart hover
        # updateCartHover cart

        $('.sqs-pill-shopping-cart-content').animate opacity: 1, 300
    , 300

  # Get selected options
  getSelectedOptions: ->
    options = []
    missing = []

    $('.variant-option').each (i, v) ->
      $(v).find('select').each (i, v) ->
        $select = $(v)
        name = $select.data('variant-option-name')
        value = $select.val()
        options[name] = value
        missing.push name if value is 'none'
        return
      return

    return [options, missing]

  # get currently selected variant or show an alert
  getVariant: ->
    [options, missing] = @getSelectedOptions()

    # Warn if missing options (we'll be unable to figure out a SKU).
    if missing.length > 0
      alert = app.get 'alert'
      alert.show
        title:   'Unable To Add Item'
        message: 'Please select a ' + missing[0] + ' option.'
        confirm: 'Okay'
      return

    getVariant options

module.exports = ProductView
