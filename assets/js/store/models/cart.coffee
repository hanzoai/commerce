ModelEmitter = require 'mvstar/lib/model-emitter'

validNum = (v) ->
  # yes javascript YES YESSSSS YESSsssssssss
  typeof v == 'number' and not isNaN v

neverBelowZero = (v) ->
  if v < 0 then 0 else v

cookies =
  get: (name) ->
    try
      state = (JSON.parse $.cookie name) ? {}
    catch
      {}

  set: (name, state, path, expires) ->
    domain = null
    unless location.hostname is 'localhost'
      domain = (location.hostname.replace 'store.', '').replace 'checkout.', ''

    $.cookie name, (JSON.stringify state),
      domain:  domain
      expires: expires
      path:    path

class Cart extends ModelEmitter
  cookieName: 'SKULLYCart'

  defaults:
    subtotal: 0
    quantity: 0
    products: {}

  validators:
    quantity: validNum
    subtotal: validNum

  transforms:
    quantity: neverBelowZero
    subtotal: neverBelowZero

  fetch: ->
    @update cookies.get @cookieName

  save: ->
    cookies.set @cookieName, @state, '/', 30

  set: (k, v) ->
    super
    @emit k, v
    console.log 'cart:', k, v

  getProduct: (sku) ->
    @state.products[sku]

  getProducts: ->
    @state.products

  _setProduct: (sku, product) ->
    @state.products[sku] = product
    @save()

  setProduct: (sku, product) ->
    product = $.extend {}, product
    quantity = @get 'quantity'
    subtotal = @get 'subtotal'

    unless @state.products[sku]?
      console.log 'new sku'
      # new sku
      @set 'quantity', quantity + product.quantity
      @set 'subtotal', subtotal + (product.quantity * product.price)
      @_setProduct sku, product
    else
      console.log 'update existing'
      # update based on existing sku
      _product = @state.products[sku]
      console.log product, _product
      quantityDiff = product.quantity - _product.quantity
      subtotalDiff = (product.quantity * product.price) - (_product.quantity * _product.price)
      console.log 'quantityDiff', quantityDiff, 'subtotalDiff', subtotalDiff
      @set 'quantity', quantity + quantityDiff
      @set 'subtotal', subtotal + subtotalDiff
      @_setProduct sku, product

  addProduct: (sku, product) ->
    product = $.extend {}, product

    # update quantity and subtotal
    price    = product.quantity * product.price
    quantity = (@get 'quantity') + product.quantity
    subtotal = (@get 'subtotal') + price

    # track add to cart conversion
    window._fbq?.push ['track', '6018312116522', {'value': price.toFixed(), 'currency':'USD'}]

    @set 'quantity', quantity
    @set 'subtotal', subtotal

    # save new item to cart
    unless @state.products[sku]?
      return @_setProduct sku, product

    # update quantity of product in cart
    @state.products[sku].quantity += product.quantity
    @save()

  removeProduct: (sku) ->
    product = @state.products[sku]

    product.quantity ?= 0
    product.price    ?= 0

    quantity = @get 'quantity'
    subtotal = @get 'subtotal'
    @set 'quantity', quantity - product.quantity
    @set 'subtotal', subtotal - (product.quantity * product.price)

    delete @state.products[sku]
    @save()

  clear: ->
    $.removeCookie @cookieName
    @state = {}
    @setDefaults()
    @set 'quantity', 0
    @set 'subtotal', 0
    @save()

module.exports = Cart
