View = require 'mvstar/lib/view'
products = require '../../utils/products'

class ProductView extends View
  el: '.product-text'

  constructor: ->
    super
    @slug = @el.data 'slug'

  events:
    'click .add-to-cart': 'addToCart'

  addToCart: ->
    unless (variant = @getVariant())?
      return

    quantity = parseInt @el.find('select[name=quantity]').val(), 10

    inner = $('.sqs-add-to-cart-button-inner')
    inner.html ''
    inner.append '<div class="yui3-widget sqs-spin light"></div>'
    inner.append '<div class="status-text">Adding...</div>'

    product = allProducts[@slug]

    app.get('cart').addProduct @slug,
      sku:      variant.SKU
      color:    variant.Color
      img:      product.Images[0].Url
      name:     product.Title
      price:    parseInt(variant.Price, 10) * 0.0001
      quantity: quantity
      size:     variant.Size
      slug:     @slug

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
    options = {}
    missing = []

    @el.find('select').each (i, v) ->
      $select = $(v)
      name = $select.attr('name')
      return if name == 'quantity'  # Not variant option

      value = $select.val()
      options[name] = value
      missing.push name if value is 'none'
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

    products.getVariant @slug, options

module.exports = ProductView
