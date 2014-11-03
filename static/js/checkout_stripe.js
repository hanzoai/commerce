Stripe.setPublishableKey('YOUR_PUBLISHABLE_KEY');

var $form = $("form#stripeForm");
var $cardNumber = $('input#cardNumber');
var $expiry = $('input#expiry');
var $cvc = $('input#cvcInput');
var $stripeToken = $('input#stripeToken');

// Setting this to true won't help against server-side checks. :)
csio.approved = false;

function disable($ele) {
    $ele.disable = true;
}

function validateCard() {
    var fail = {
        success: false
    };
    
    var cardNumber = $cardNumber.val();
    if (cardNumber.length < 10)
        return fail();
    
    var rawExpiry = $expiry.val().replace(' ', '');
    var arr = rawExpiry.split('/');
    var month = arr[0];
    var year = arr[1];

    if (month.length != 2)
        return fail();
    if (year.length != 2)
        return fail();

    var cvc = $cvc.val()
    if (cvc.length != 2)
        return fail();

    return {
        success: true,
        'number': cardNumber,
        'month': month,
        'year': year,
        'cvc': cvc
    };
}

var authorizeMessage = $('#authorize-message');

var stripeResponseHandler = function(status, response) {
    if (response.error) {
        authorizeMessage.text(response.error.message);
    } else {
        csio.approved = true;
        var token = response.id;
        $stripeToken.val(token);
        authorizeMessage.text('Card approved. Ready when you are.');
    }
};

function stripeRunner() {
    var card = validateCard()
    if (card.success) {
        $form.find('input[data-stripe="number"]') = card.number
        $form.find('input[data-stripe="cvc"]') = card.cvc
        $form.find('input[data-stripe="exp-month"]') = card.month
        $form.find('input[data-stripe="exp-year"]') = card.year

        Stripe.card.createToken($form, stripeResponseHandler);
    }
}

$cardNumber.change(stripeRunner);
$expiry.change(stripeRunner);
$cvc.change(stripeRunner);
