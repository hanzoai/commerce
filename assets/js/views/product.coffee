view = require '../view'

class Product extends View
  addProduct: ->
    quantity = parseInt($("#quantity").val(), 10)
    cart = @get()
    variant = product.getVariant()

    return unless variant?

    sku = variant.SKU

    if cart[sku]
      cart[sku].quantity += quantity
    else
      cart[sku] =
        sku: variant.SKU
        color: variant.Color
        img: csio.currentProduct.Images[0].Url
        name: csio.currentProduct.Title
        quantity: quantity
        size: variant.Size
        price: parseInt(variant.Price, 10) * 0.0001
        slug: csio.currentProduct.Slug

    # Set cookie
    csio.setCart cart

    inner = $(".sqs-add-to-cart-button-inner")
    inner.html ""
    inner.append "<div class=\"yui3-widget sqs-spin light\" ></div>"
    inner.append "<div class=\"status-text\">Adding...</div>"

    setTimeout ->
      $(".status-text").text("Added!").fadeOut 500, ->
        inner.html "Add to Cart"

    , 500

    setTimeout ->
      # Flash cart hover
      $(".sqs-pill-shopping-cart-content").animate opacity: 0.85, 400, ->
        # Update cart hover
        csio.updateCartHover cart

        $(".sqs-pill-shopping-cart-content").animate opacity: 1, 300

    , 300
