AlertView = require './alert'
View = require '../view'

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

  getVariant: ->
    selected = {}
    variants = currentProduct.Variants
    missingOptions = []

    # Determine if selected options match variant
    optionsMatch = (selected, variant) ->
      for k,v of selected
        if variant[k] != selected[k]
          return false
      true

    # Get selected options
    $(".variant-option").each (i, v) ->
      $(v).find("select").each (i, v) ->
        $select = $(v)
        name = $select.data("variant-option-name")
        value = $select.val()
        selected[name] = value
        missingOptions.push name if value is 'none'
        return

      return

    # Warn if missing options (we'll be unable to figure out a SKU).
    if missingOptions.length > 0
      alert = new AlertView nextTo: '.sqs-add-to-cart-button'
      alert.show
        title:   'Unable To Add Item'
        message: 'Please select a ' + missingOptions[0] + ' option.'
        confirm: 'Okay'
      return

    # Figure out SKU
    for variant in variants
      # All options match match variant
      return variant if optionsMatch selected, variant

    # Only one variant, no options.
    variants[0]

module.exports = ProductView
