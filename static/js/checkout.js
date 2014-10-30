var validation = {
    isEmpty: function (str) {
        return str.trim().length == 0;
    },
    isEmail: function(email) {
        var pattern = new RegExp(/^[+a-zA-Z0-9._-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,4}$/i);
        return pattern.test(email);
    }
}
    

$('div.field').on('click', function() {
    $(this).removeClass('error');
});

$visibleRequired = $('div:visible.required > input');

$('#form').submit(function(e) {
    var empty = $visibleRequired.filter(function() {return validation.isEmpty($(this).val())});

    var email = $('input[name="User.Email"]')
    if (!validation.isEmail(email)) {
        email.parent().addClass('error')
        e.preventDefault()
    }
    
    if (empty.length > 0) {
        e.preventDefault();
        empty.parent().addClass('error');
    }
});

$visibleRequired.change(function(){
    var empty = $visibleRequired.filter(function() {return $(this).val().trim() == "";});

    if (empty.length == 0) {
        var fieldset = $('div.sqs-checkout-form-payment-content > fieldset');
        fieldset.css('display', 'block');
        fieldset.css('opacity', '0');
        fieldset.fadeTo(1000, 1);
    }
});

$('#form').card({
    container: '#card-wrapper',
    numberInput: 'input[name="Order.Account.Number"]',
    expiryInput: 'input[name="RawExpiry"]',
    cvcInput: 'input[name="Order.Account.CVV2"]',
    nameInput: 'input[name="User.FirstName"], input[name="User.LastName"]'
});

var firstTime = true
$('input[name="ShipToBilling"]').change(function(){
    firstTime = false
    var shipping = $('#shippingInfo')
    if (this.checked && !firstTime) {
        shipping.fadeTo(500, 0);
        setTimeout(function(){
            shipping.css('display', 'none');
        }, 500);
    } else {
        shipping.fadeTo(500, 1);
        shipping.css('display', 'block');
    }
});
