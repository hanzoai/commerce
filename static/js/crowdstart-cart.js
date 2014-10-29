csio = window.csio || {};
var templateEl = $('.template');
templateEl.parent().remove();

csio.renderLineItem = function(lineItem, index) {
    var el = templateEl.clone(false);
    var $quantity = el.find('.quantity input');

    // get list of variants
    var variantInfo = []
    if (lineItem.color !== "")
      variantInfo.push(lineItem.color)

    if (lineItem.size !== "")
      variantInfo.push(lineItem.size)

    el.find('img.thumbnail').attr('src', lineItem.img);
    el.find('input.slug').val(lineItem.slug).attr('name', 'Order.Items.' + index + '.Product.Slug');
    el.find('input.sku').val(lineItem.sku).attr('name', 'Order.Items.' + index + '.Variant.SKU');
    el.find('a.title').text(lineItem.name);
    el.find('div.variant-info').text(variantInfo.join(' / '));
    el.find('.price span').text(formatCurrency(lineItem.price));
    $quantity.val(lineItem.quantity).attr('name', 'Order.Items.' + index + '.Quantity');

    // Handle quantity changes
    $quantity.change(function(e) {
      e.preventDefault();
      e.stopPropagation();

      // Get quantity
      var quantity = $(this).val()

      // Prevent less than one quantity
      if (quantity < 1) {
        quantity = 1
        $(this).val(1)
      }

      // Update quantity
      lineItem.quantity = quantity

      // Update line item
      csio.updateLineItem(lineItem, el);
    })

    // Handle lineItem removals
    el.find('.remove-item').click(function() {
      csio.removeLineItem(lineItem.sku, el);
    });

    el.removeClass('template');

    $('.cart-container tbody').append(el);
};

csio.renderCart = function(modifiedCart) {
  var cart = modifiedCart || csio.getCart();
  var numItems = 0;
  var subtotal = 0;
  var i = 0;

  $('.cart-container tbody').html('');

  for (var k in cart) {
    var lineItem = cart[k];
    numItems += lineItem.quantity;
    subtotal += lineItem.price * lineItem.quantity;
    csio.renderLineItem(lineItem, i);
    i += 1;
  }

  if (i === 0) {
    $('.cart-container').hide();
    $('.empty-message').show();
  } else {
    csio.updateSubtotal(subtotal)
  }
};

csio.getSubtotal = function() {
    var subtotal = 0
    var cart = csio.getCart();
    for (var k in cart) {
      subtotal += cart[k].quantity * cart[k].price
    }
    return subtotal
}

csio.updateSubtotal = function(subtotal) {
  var subtotal = subtotal || csio.getSubtotal();

  $('.subtotal .price span').text(formatCurrency(subtotal));
}

csio.removeLineItem = function(sku, el) {
  var cart = csio.getCart();

  delete cart[sku];

  csio.setCart(cart);
  csio.updateSubtotal();
  $(el).remove()
};

csio.updateLineItem = function(lineItem, el) {
  var cart = csio.getCart();

  cart[lineItem.sku] = lineItem;
  csio.setCart(cart);
  csio.updateSubtotal();
};

csio.renderCart();

$('input,select').keypress(function(e) { return e.keyCode != 13; });
