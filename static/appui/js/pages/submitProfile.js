/*
 *  Document   : submitProfile.js
 *  Author     : verusmedia
 *  Description: Custom javascript code used on Profile page
 */

var SubmitProfile = function() {

    return {
        init: function() {
            /*
             *  Jquery Validation, Check out more examples and documentation at https://github.com/jzaefferer/jquery-validation
             */

            /* Login form - Initialize Validation */
            $('#form-profile').validate({
                errorClass: 'help-block animation-slideUp', // You can change the animation class for a different entrance animation - check animations page
                errorElement: 'div',
                errorPlacement: function(error, e) {
                    e.parents('.form-group > div').append(error);
                },
                highlight: function(e) {
                    $(e).closest('.form-group').removeClass('has-success has-error').addClass('has-error');
                    $(e).closest('.help-block').remove();
                },
                success: function(e) {
                    e.closest('.form-group').removeClass('has-success has-error');
                    e.closest('.help-block').remove();
                },
                rules: {
                    'User.FirstName': {
                        required: true,
                    },
                    'User.LastName': {
                        required: true,
                    },
                    'User.Email': {
                        required: true,
                        email: true
                    },
                },
                messages: {
                    'User.FirstName': 'Please enter your first name',
                    'User.LastName': 'Please enter your last name',
                    'User.Email': 'Please enter your email',
                }
            });
        }
    };
}();

