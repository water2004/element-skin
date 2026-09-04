interface ValidatableForm {
  validate: () => Promise<boolean>
}

export async function validateForm(form: ValidatableForm | null): Promise<boolean> {
  if (!form) return false

  try {
    return await form.validate()
  } catch {
    return false
  }
}
