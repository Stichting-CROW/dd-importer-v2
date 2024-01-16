package feed_status

func updateStatus() {
	stmt := `INSERT INTO customers (name, email)
	VALUES('Microsoft','hotline@microsoft.com') 
	ON CONFLICT (name) 
	DO 
	   UPDATE SET email = EXCLUDED.email || ';' || customers.email;`
}
