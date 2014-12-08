View = require 'mvstar/lib/view'
products = require '../../utils/products'

class ProductView extends View
  el: '.product-text'

  bindings:
    productListing: '.product-cost .money'

  formatters:
    productListing: (v)->
      cost = 0
      for config in v.ProductConfigs

        product = allProducts[config.Product]
        if !product
          break

        variants = product.Variants
        if !variants
          break

        if config.Variant != ''
          variant = variants.some (val)->
            if val.Id == config.variant
              cost += parseInt(variant.Price, 10)
              return true
            return false
        else
          cost += variants.reduce (last, current)->
            return Math.min(last, current.Price)
          , Number.MAX_VALUE
      @cost = cost
      return (cost * .0001).toFixed(2) + ""

  events:
    'click .add-to-cart': 'addToCart'

  render: ->
    @slug = @el.data 'slug'
    @set 'productListing', allProductListings[@slug]
    super

  addToCart: ->
    unless (variant = @getVariant())?
      return

    quantity = parseInt @el.find('select[name=quantity]').val(), 10

    inner = @el.find 'span'
    inner.html ''
    inner.append '<div class="loading-spinner" style="float:left"></div>'
    inner.append '<div class="add-to-cart-adding-text" style="float:right">Adding...</div>'

    productListing = @get 'productListing'

    # Refuse to add more than 99 items to the cart
    cart = app.get 'cart'
    if (cart.getProduct variant.SKU)?.quantity + quantity > (app.get 'maxQuantityPerProduct')
      setTimeout =>
        @el.find('span').text("Too Many").fadeOut 1000, =>
          inner.html 'Add to Cart'
          @el.find('span').fadeIn()
      , 500
      return

    cart.addProduct variant.SKU,
      sku:      variant.SKU
      color:    variant.Color
      img:      product.Images[0].Url
      name:     product.Title
      price:    parseInt(variant.Price, 10) * 0.0001
      quantity: quantity
      size:     variant.Size
      slug:     @slug

    setTimeout =>
      @el.find('span').text('Added!').fadeOut 500, =>
        inner.html 'Add to Cart'
        @el.find('span').fadeIn()
    , 500

    # Flash cart hover
    setTimeout ->
      $('.cart-hover').animate opacity: 1, 400, ->
        $('.cart-hover').animate opacity: 0.9, 300
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
