View = require 'mvstar/lib/view'
products = require '../../utils/products'

class ProductView extends View
  el: '.product-text'

  bindings:
    listing: '.product-cost .money'

  formatters:
    listing: (v)->
      cost = 0
      for config in v.Configs

        product = allProducts[config.Product]
        if !product
          break

        variants = product.Variants
        if !variants
          break

        if config.Variant != ''
          variant = variants.some (val)->
            if val.Id == config.Variant
              cost += parseInt(variant.Price, 10)
              return true
            return false
        else
          cost += variants.reduce (last, current)->
            return Math.min(last, current.Price)
          , Number.MAX_VALUE

        cost -= parseInt(config.PriceAdjustment, 10)

      @cost = cost
      return (cost * .0001).toFixed(2) + ""

  events:
    'click .add-to-cart': 'addToCart'

  render: ->
    sku = @el.data 'sku'
    console.log sku
    listing = allListings[sku]
    console.log listing
    @set 'listing', allListings[sku]
    super

  addToCart: ->
    #So the first item of a configuration will be the 'root' product displayed
    #All variant info is for this 'root' product
    #All other product will be 'children' and should be fixed variants
    listing = @get 'listing'
    slug = listing.Configs[0].Product
    unless (variant = @getVariant(slug))?
      return

    listingSKU = listing.SKU + variant.SKU

    quantity = parseInt @el.find('select[name=quantity]').val(), 10

    product = allProducts[slug]
    childProducts = []
    rootProduct =
      listingSKU: listingSKU
      sku:      variant.SKU
      color:    variant.Color
      img:      product.Images?[0]?.Url
      name:     product.Title
      price:    (parseInt(variant.Price, 10) - parseInt(listing.Configs[0].PriceAdjustment, 10)) * 0.0001
      quantity: quantity
      size:     variant.Size
      slug:     product.Slug
      children: childProducts

    for config in listing.Configs
      product = allProducts[config.Product]
      variant = products.getVariant(config.Product, {}) #supply blank options temporarily
      childProducts.push
        sku:      variant.SKU
        color:    variant.Color
        img:      product.Images?[0]?.Url
        name:     product.Title
        price:    (parseInt(variant.Price, 10) - parseInt(config.PriceAdjustment, 10)) * 0.0001
        multiplier: config.Quantity
        size:     variant.Size
        slug:     product.Slug

    inner = @el.find '.add-to-cart span'
    inner.html ''
    inner.append '<div class="loading-spinner" style="float:left"></div>'
    inner.append '<div class="add-to-cart-adding-text" style="float:right">Adding...</div>'

    listing = @get 'listing'

    # Refuse to add product to cart if total in cart would exceed maxQuantityPerProduct.
    cart = app.get 'cart'
    if (cart.getProduct listingSKU)?.quantity + quantity > (app.get 'maxQuantityPerProduct')
      setTimeout =>
        @el.find('.add-to-cart span').text("Too Many").fadeOut 1000, =>
          inner.html 'Add to Cart'
          @el.find('.add-to-cart span').fadeIn()
      , 500
      return

    # for each listing SKU, we list the product configuration
    cart.addProduct listingSKU, rootProduct

    setTimeout =>
      @el.find('.add-to-cart span').text('Added!').fadeOut 500, =>
        inner.html 'Add to Cart'
        @el.find('.add-to-cart span').fadeIn()
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
  getVariant: (slug)->
    [options, missing] = @getSelectedOptions()

    # Warn if missing options (we'll be unable to figure out a SKU).
    if missing.length > 0
      alert = app.get 'alert'
      alert.show
        title:   'Unable To Add Item'
        message: 'Please select a ' + missing[0] + ' option.'
        confirm: 'Okay'
      return

    products.getVariant slug, options

module.exports = ProductView
