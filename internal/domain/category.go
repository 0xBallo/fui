package domain

// CategoryKind says whether a category classifies income or expenses.
type CategoryKind string

// Category kinds.
const (
	CategoryIncome  CategoryKind = "income"
	CategoryExpense CategoryKind = "expense"
)

// Valid reports whether k is a known category kind.
func (k CategoryKind) Valid() bool { return k == CategoryIncome || k == CategoryExpense }

// ParseCategoryKind parses a category kind (case-insensitive).
func ParseCategoryKind(s string) (CategoryKind, error) {
	return parseEnum("category kind", s, CategoryKind.Valid)
}

// Category classifies transactions. There is a single level: no subcategories
// (finer detail is the job of tags).
type Category struct {
	ID   CategoryID
	Name string
	Kind CategoryKind
}

// NewCategory validates and returns a category with the name trimmed.
func NewCategory(name string, kind CategoryKind) (Category, error) {
	n, err := requireName("name", name)
	if err != nil {
		return Category{}, err
	}
	if !kind.Valid() {
		return Category{}, enumError("kind", string(kind))
	}
	return Category{Name: n, Kind: kind}, nil
}

// DefaultCategories returns the categories `fui init` creates, in display order.
func DefaultCategories() []Category {
	expenses := []string{
		"Housing", "Groceries", "Dining", "Transport", "Utilities", "Health", "Insurance", "Education",
		"Entertainment", "Shopping", "Travel", "Subscriptions", "Taxes", "Gifts", "Fees", "Other Expenses",
	}
	income := []string{"Salary", "Freelance", "Interest", "Other Income"}
	cats := make([]Category, 0, len(expenses)+len(income))
	for _, n := range expenses {
		cats = append(cats, Category{Name: n, Kind: CategoryExpense})
	}
	for _, n := range income {
		cats = append(cats, Category{Name: n, Kind: CategoryIncome})
	}
	return cats
}
