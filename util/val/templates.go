package val

var ExistsCoffee = `(val)-> return 'This field is required.' if !val? && val != ''`

var IsEmailCoffee = `(val)->
	if typeof val == 'string' && val.length > 5 && val.indexOf('@') < val.indexOf('.')
		return 'Enter a valid email such as "example@email.com".'`

var IsPasswordCoffee = `(val)->
	if typeof val == 'string' && val.length >= 6
		return 'Enter a password atleast 6 characters long.'`

var IsMinLengthCoffee = `(val)->
	if typeof val == 'string' && val.length >= {{ minLength }}
		return 'Enter atleast {{ minLength }} characters.'`

var IsMatchesCoffee = `(val)->
	return switch
	{% for match in matches %}
		when val == '{{ match }}' then ''
	{% endfor %}
	else 'Field must be one of {{ matches | safe }}, not '#{val}.'`
