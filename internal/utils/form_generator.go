package utils

import (
	"bytes"
	"fmt"
	"go-web/internal/config"
)

func GenerateFormHTML(form *config.FormConfig) (string, error) {
	var buf bytes.Buffer

	buf.WriteString("<!DOCTYPE html>\n<html>\n<head>\n")
	buf.WriteString("    <meta charset='UTF-8'>\n")
	buf.WriteString(fmt.Sprintf("    <title>%s</title>\n", form.Title))
	buf.WriteString(`    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; background: #f5f7fa; padding: 20px; }
        .container { max-width: 800px; margin: 0 auto; background: white; padding: 30px; border-radius: 10px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
        h1 { color: #2c3e50; margin-bottom: 10px; text-align: center; }
        .description { color: #7f8c8d; margin-bottom: 30px; text-align: center; }
        .form-group { margin-bottom: 20px; }
        label { display: block; margin-bottom: 8px; color: #2c3e50; font-weight: 500; }
        input, select, textarea { width: 100%; padding: 12px; border: 1px solid #ddd; border-radius: 5px; font-size: 14px; transition: border-color 0.3s; }
        input:focus, select:focus, textarea:focus { outline: none; border-color: #3498db; }
        .required { color: #e74c3c; margin-left: 5px; }
        .checkbox-group { display: flex; flex-wrap: wrap; gap: 10px; }
        .checkbox-item { display: flex; align-items: center; }
        .checkbox-item input { margin-right: 5px; }
        button { width: 100%; padding: 15px; background: #3498db; color: white; border: none; border-radius: 5px; font-size: 16px; cursor: pointer; transition: background 0.3s; margin-top: 10px; }
        button:hover { background: #2980b9; }
        .success { color: #27ae60; text-align: center; padding: 20px; background: #e8f8f5; border-radius: 5px; margin-top: 20px; }
    </style>
</head>
<body>
    <div class="container">
        <h1>`)

	buf.WriteString(form.Title)
	buf.WriteString(`</h1>
        <p class="description">` + form.Description + `</p>

        <form id="dataForm">
`)

	for _, field := range form.Fields {
		buf.WriteString(fmt.Sprintf("        <div class='form-group'>\n            <label>%s", field.Label))
		if field.Required {
			buf.WriteString(`<span class="required">*</span>`)
		}
		buf.WriteString("</label>\n")

		switch field.Type {
		case "text":
			buf.WriteString(fmt.Sprintf("            <input type='text' name='%s' placeholder='%s'%s>\n",
				field.Name, field.Placeholder, requiredAttr(field.Required)))
		case "email":
			buf.WriteString(fmt.Sprintf("            <input type='email' name='%s' placeholder='%s'%s>\n",
				field.Name, field.Placeholder, requiredAttr(field.Required)))
		case "tel":
			buf.WriteString(fmt.Sprintf("            <input type='tel' name='%s' placeholder='%s'%s>\n",
				field.Name, field.Placeholder, requiredAttr(field.Required)))
		case "number":
			buf.WriteString(fmt.Sprintf("            <input type='number' name='%s' placeholder='%s'%s",
				field.Name, field.Placeholder, requiredAttr(field.Required)))
			if field.Min != nil {
				buf.WriteString(fmt.Sprintf(" min='%f'", *field.Min))
			}
			if field.Max != nil {
				buf.WriteString(fmt.Sprintf(" max='%f'", *field.Max))
			}
			buf.WriteString(">\n")
		case "textarea":
			buf.WriteString(fmt.Sprintf("            <textarea name='%s' placeholder='%s'%s rows='4'></textarea>\n",
				field.Name, field.Placeholder, requiredAttr(field.Required)))
		case "select":
			buf.WriteString(fmt.Sprintf("            <select name='%s'%s>\n                <option value=''>请选择</option>\n",
				field.Name, requiredAttr(field.Required)))
			for _, option := range field.Options {
				buf.WriteString(fmt.Sprintf("                <option value='%s'>%s</option>\n", option, option))
			}
			buf.WriteString("            </select>\n")
		case "checkbox":
			buf.WriteString("            <div class='checkbox-group'>\n")
			for _, option := range field.Options {
				buf.WriteString(fmt.Sprintf("                <label class='checkbox-item'><input type='checkbox' name='%s[]' value='%s'> %s</label>\n",
					field.Name, option, option))
			}
			buf.WriteString("            </div>\n")
		case "radio":
			buf.WriteString("            <div class='radio-group'>\n")
			for _, option := range field.Options {
				buf.WriteString(fmt.Sprintf("                <label><input type='radio' name='%s' value='%s'> %s</label>\n",
					field.Name, option, option))
			}
			buf.WriteString("            </div>\n")
		case "date":
			buf.WriteString(fmt.Sprintf("            <input type='date' name='%s'%s>\n",
				field.Name, requiredAttr(field.Required)))
		case "time":
			buf.WriteString(fmt.Sprintf("            <input type='time' name='%s'%s>\n",
				field.Name, requiredAttr(field.Required)))
		default:
			buf.WriteString(fmt.Sprintf("            <input type='text' name='%s' placeholder='%s'%s>\n",
				field.Name, field.Placeholder, requiredAttr(field.Required)))
		}

		buf.WriteString("        </div>\n")
	}

	buf.WriteString(`        <button type="submit">提交</button>
        </form>

        <div id="successMessage" class="success" style="display: none;">
            <h3>✅ 提交成功！</h3>
            <p>感谢您的参与！</p>
        </div>
    </div>

    <script>
        document.getElementById('dataForm').addEventListener('submit', async function(e) {
            e.preventDefault();
            
            const formData = new FormData(this);
            const data = {};
            
            for (let [key, value] of formData.entries()) {
                if (data[key]) {
                    if (Array.isArray(data[key])) {
                        data[key].push(value);
                    } else {
                        data[key] = [data[key], value];
                    }
                } else {
                    data[key] = value;
                }
            }
            
            try {
                const response = await fetch('/api/submit/` + form.Name + `', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(data)
                });
                
                if (response.ok) {
                    document.getElementById('successMessage').style.display = 'block';
                    this.reset();
                } else {
                    const error = await response.json();
                    alert('提交失败: ' + error.message);
                }
            } catch (err) {
                alert('网络错误: ' + err.message);
            }
        });
    </script>
</body>
</html>`)

	return buf.String(), nil
}

func requiredAttr(required bool) string {
	if required {
		return " required"
	}
	return ""
}

func Version() string {
	return "1.0.0"
}
