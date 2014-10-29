csio = window.csio || {};
var templateEl = $('.template');
templateEl.parent().remove();

csio.renderLineItem = function(lineItem, index) {
    var cartEl = templateEl.clone(false);

    cartEl.find('img.thumbnail').attr('src', lineItem.img);
    cartEl.find('input.slug').val(lineItem.slug).attr('name', 'Order.Items.' + index + '.Product.Slug');
    cartEl.find('input.sku').val(lineItem.sku).attr('name', 'Order.Items.' + index + '.Variant.SKU');
    cartEl.find('a.title').text(lineItem.name);
    cartEl.find('div.variant-info').text([lineItem.color, lineItem.size].join(' / '));
    cartEl.find('.quantity input').val(lineItem.quantity).attr('name', 'Order.Items.' + index + '.Quantity');
    cartEl.find('.price span').text(formatCurrency(lineItem.price));

    cartEl.find('.remove-item').click(function() {
      csio.removeLineItem(lineItem.sku);
    });

    cartEl.removeClass('template');

    $('.cart-container tbody').append(cartEl);
};

csio.renderCart = function(modifiedCart) {
  var cart = modifiedCart || csio.getCart();
  var numItems = 0;
  var subTotal = 0;
  var i = 0;

  for (var k in cart) {
    var lineItem = cart[k];
    numItems += lineItem.quantity;
    subTotal += lineItem.price * lineItem.quantity;
    csio.renderLineItem(lineItem, i);
    i += 1;
  }

  if (i === 0) {
    $('.cart-container').hide();
    $('.empty-message').show();
  } else {
    $('.subtotal .price span').text(formatCurrency(price));
  }
};

csio.removeLineItem = function(sku) {
  var cart = csio.getCart();

  delete cart[sku];

  csio.setCart(cart);
  csio.renderCart(cart);
};

csio.renderCart();
